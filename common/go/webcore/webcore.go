package webcore

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name" form:"name" query:"name"`
	FullName  string    `json:"full_name" form:"full_name" query:"full_name"`
	Company   string    `json:"company" form:"company" query:"company"`
	Address   string    `json:"address" form:"address" query:"address"`
	Phone     string    `json:"phone" form:"phone" query:"phone"`
	Email     string    `json:"email" form:"email" query:"email"`
	Token     string    `json:"token" form:"token" query:"token"`
	AvatarURL string    `json:"avatar_url"`
	CreatedAt time.Time `json:"created_at"`
	ApiKey    string    `json:"api_key"`
}

type OAuthClientConfig struct {
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
	RedirectURI       string `json:"redirect_uri"`
	MockLogin         string `json:"mock_login"`
}

type StripeConfig struct {
	Secret string `json:"secret"`
	Public string `json:"public"`
}

type SMTPConfig struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type Config struct {
	Addr        string            `json:"addr"`
	WebRootDir  string            `json:"web_root"`
	Dir         string            `json:"dir"`
	Title       string            `json:"title"`
	GitHubOAuth OAuthClientConfig `json:"github_oauth"`
	GoogleOAuth OAuthClientConfig `json:"google_oauth"`
	EmailAuth   bool              `json:"email_auth"`
	Stripe      StripeConfig      `json:"stripe"`
	AdminUsers  []string          `json:"admin_users"`
	SMTP        SMTPConfig        `json:"smtp"`
}

type App interface {
	GetUsers() map[string]*User
	GetWebcoreConfig() *Config
	GetAppConfig() interface{}
	GetDB() interface{}
	Dblock()
	Dbunlock()
	InitEndpoints()
}

var (
	configFile = flag.String("config-file", "liman_config.json", "Configuration file")
	BuildID    string
)

func UUID() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	escaper := strings.NewReplacer("9", "99", "-", "90", "_", "91")
	return escaper.Replace(base64.RawURLEncoding.EncodeToString(buf))
	// return base32.StdEncoding.EncodeToString(buf) //base32.EncodeToString(buf)
}

func ReplyJSON(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func ApiError(w http.ResponseWriter, format string, a ...interface{}) error {
	type e struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	s := struct {
		Error e `json:"error"`
	}{e{fmt.Sprintf(format, a), http.StatusInternalServerError}}
	log.Println("ERROR:", s.Error.Message)
	return ReplyJSON(w, s.Error.Code, s)
}

func getUser(a App, token string) *User {
	a.Dblock()
	defer a.Dbunlock()
	for _, u := range a.GetUsers() {
		if u.Token == token || u.ApiKey == token {
			return u
		}
	}
	return nil
}

func GetBearerAuth(r *http.Request) string {
	hdr := r.Header.Get("Authorization")
	parts := strings.Split(hdr, "Bearer ")
	tok := ""
	if len(parts) > 1 {
		tok = parts[1]
	}
	return tok
}

func GetUser(a App, r *http.Request) *User {
	tok := GetBearerAuth(r)
	if tok == "" {
		if r.FormValue("access_token") != "" {
			tok = r.FormValue("access_token")
		} else if cookie, err := r.Cookie("access_token"); err == nil {
			tok = cookie.Value
		}
	}
	return getUser(a, tok)
}

func Auth(a App, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := GetUser(a, r)
		if u != nil {
			ctx := context.WithValue(r.Context(), "user", u)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}
}

func IsAdminUser(a App, u *User) bool {
	for _, entry := range a.GetWebcoreConfig().AdminUsers {
		if u != nil && entry == u.ID {
			return true
		}
	}
	return false
}

func AdminAuth(a App, next http.HandlerFunc) http.HandlerFunc {
	return Auth(a, func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value("user").(*User)
		if u != nil {
			next.ServeHTTP(w, r)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// Request executes an HTTP request with the given method, URL and body/query parameters.
// It expects a JSON response and unmarshals it into an arbitrary result structure.
func Request(method, url string, values url.Values, user, passwd string, result interface{}) error {
	body, err := RequestRaw(method, url, values, user, passwd)
	if err != nil {
		return nil
	}
	return json.Unmarshal(body, result)
}

func RequestRaw(method, url string, values url.Values, user, passwd string) ([]byte, error) {
	var r io.Reader
	if values != nil {
		if method == "GET" {
			url = url + "?" + values.Encode()
		} else {
			r = strings.NewReader(values.Encode())
		}
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return nil, err
	}

	if user != "" || passwd != "" {
		req.SetBasicAuth(user, passwd)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func saveDB(a App) error {
	a.Dblock()
	str, _ := json.MarshalIndent(a.GetDB(), "", "  ")
	a.Dbunlock()
	fname := filepath.Join(a.GetWebcoreConfig().Dir, "db.json")
	backupDir := filepath.Join(a.GetWebcoreConfig().Dir, "backups")
	tmp := fname + "." + UUID()
	if err := ioutil.WriteFile(tmp, str, 0775); err != nil {
		str := fmt.Sprintf("Error writing to %s: %v\n", tmp, err)
		log.Println(str)
		return errors.New(str)
	}
	os.MkdirAll(backupDir, 0775)
	suffix := time.Now().Format("2006-01-02.15:04:05")
	backupFileName := filepath.Join(backupDir, filepath.Base(fname)+"."+suffix)
	if err := os.Rename(fname, backupFileName); err != nil {
		log.Printf("Error rename(%s -> %s): %v\n", fname, backupFileName, err)
	}
	if err := os.Rename(tmp, fname); err != nil {
		log.Printf("Error rename(%s -> %s): %v\n", tmp, fname, err)
	}
	log.Printf("DB saved to %s, backup %s\n", fname, backupFileName)
	return nil
}

func InitEndpoints(a App) {

	mkuser := func(id, token, name, avatarURL string) *User {
		users := a.GetUsers()
		a.Dblock()
		defer a.Dbunlock()
		u := users[id]
		if u == nil {
			u = &User{ID: id, CreatedAt: time.Now(), ApiKey: UUID()}
			users[id] = u
		}
		u.Name = name
		u.AvatarURL = avatarURL
		u.Token = token
		return u
	}

	setUser := func(w http.ResponseWriter, r *http.Request, id, token, name, avatarURL string) {
		mkuser(id, token, name, avatarURL)
		http.SetCookie(w, &http.Cookie{
			Name:    "access_token",
			Value:   token,
			Path:    "/",
			Expires: time.Now().Add(48 * time.Hour),
		})
		http.Redirect(w, r, "/", http.StatusFound)
	}

	http.HandleFunc("/oauth/github/callback", func(w http.ResponseWriter, r *http.Request) {
		token := struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error_description"`
		}{}
		code := r.FormValue("code")
		body, err := RequestRaw("POST", "https://github.com/login/oauth/access_token", url.Values{
			"client_id":     []string{a.GetWebcoreConfig().GitHubOAuth.OAuthClientID},
			"client_secret": []string{a.GetWebcoreConfig().GitHubOAuth.OAuthClientSecret},
			"redirect_uri":  []string{a.GetWebcoreConfig().GitHubOAuth.RedirectURI},
			"code":          []string{code},
		}, "", "")
		log.Printf("GH auth, code [%s] res [%s]\n", code, string(body))
		json.Unmarshal(body, &token)
		if err != nil || token.AccessToken == "" {
			ApiError(w, "Bad response from GitHub: %+v, qs [%s], err %v", token, r.URL.RawQuery, err)
			return
		}
		userInfo := struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		}{}
		log.Println("aa1")
		err = Request("GET", "https://api.github.com/user", url.Values{
			"access_token": []string{token.AccessToken},
		}, "", "", &userInfo)
		log.Println("aa2")
		if err != nil || userInfo.Login == "" {
			ApiError(w, "Failed to fetch github user data: %v", err)
			return
		}
		setUser(w, r, "github_"+userInfo.Login, token.AccessToken, userInfo.Login, userInfo.AvatarURL)
	})

	// e.GET("/oauth/google/callback", func(c echo.Context) error {
	http.HandleFunc("/oauth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		token := struct {
			AccessToken string `json:"id_token"`
		}{}
		err := Request("POST", "https://www.googleapis.com/oauth2/v4/token", url.Values{
			"client_id":     []string{a.GetWebcoreConfig().GoogleOAuth.OAuthClientID},
			"client_secret": []string{a.GetWebcoreConfig().GoogleOAuth.OAuthClientSecret},
			"grant_type":    []string{"authorization_code"},
			"redirect_uri":  []string{a.GetWebcoreConfig().GoogleOAuth.RedirectURI},
			"code":          []string{r.FormValue("code")},
		}, "", "", &token)
		if err != nil || token.AccessToken == "" {
			ApiError(w, "Bad response from Google: %+v, err %v", token, err)
			return
		}
		tokenInfo := struct {
			Email  string `json:"email"`
			UserID string `json:"user_id"`
		}{}
		err = Request("GET", "https://www.googleapis.com/oauth2/v2/tokeninfo", url.Values{
			"id_token": []string{token.AccessToken},
		}, "", "", &tokenInfo)
		if err != nil || tokenInfo.UserID == "" {
			ApiError(w, "Failed to fetch google token info: %v", err)
			return
		}

		// photodata := struct {
		// 	PhotoData string `json:"photoData"`
		// }{}
		// req, _ := http.NewRequest("GET", "https://www.googleapis.com/admin/directory/v1/users/"+tokenInfo.UserID+"/photos/thumbnail", nil)
		// req.Header.Add("Authorization", "Bearer "+token.AccessToken)
		// client := &http.Client{}
		// res, _ := client.Do(req)
		// defer res.Body.Close()
		// body, _ := ioutil.ReadAll(res.Body)
		// log.Println("RESULT:::", string(body))

		// u := "https://www.googleapis.com/admin/directory/v1/users/" + tokenInfo.UserID + "/photos/thumbnail?access_token=" + token.AccessToken
		// log.Println("URL:", u)
		// err = Request("GET", "https://www.googleapis.com/admin/directory/v1/users/"+tokenInfo.UserID+"/photos/thumbnail",
		// 	url.Values{"access_token": []string{token.AccessToken}}, "", "", &photodata)
		// if err != nil {
		// 	log.Printf("Failed to fetch google user image: %v, user %s", err, tokenInfo.Email)
		// }
		setUser(w, r, "google_"+tokenInfo.UserID, token.AccessToken, tokenInfo.Email, "")
	})

	http.HandleFunc("/api/v1/user", Auth(a, func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value("user").(*User)
		if r.Method == "GET" {
		}
		if r.Method == "POST" {
			data := User{}
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				ApiError(w, "Invalid payload: %v", err)
				return
			}
			a.Dblock()
			u.FullName = data.FullName
			u.Company = data.Company
			u.Address = data.Address
			u.Phone = data.Phone
			u.Email = data.Email
			a.Dbunlock()
		}
		ReplyJSON(w, http.StatusOK, u)
	}))

	http.HandleFunc("/api/v1/user/isadmin", Auth(a, func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value("user").(*User)
		ReplyJSON(w, http.StatusOK, IsAdminUser(a, u))
	}))

	http.HandleFunc("/api/v1/debug", AdminAuth(a, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<pre>
<a href="db">show database</a>
<a href="dbsave">save database</a>
<a href="pprof/">show profiling info</a>
</pre> Build ID :`+BuildID)
	}))

	// PROFILLING
	http.HandleFunc("/api/v1/pprof/", AdminAuth(a, pprof.Index))
	http.HandleFunc("/api/v1/pprof/heap", AdminAuth(a, pprof.Handler("heap").ServeHTTP))
	http.HandleFunc("/api/v1/pprof/goroutine", AdminAuth(a, pprof.Handler("goroutine").ServeHTTP))
	http.HandleFunc("/api/v1/pprof/block", AdminAuth(a, pprof.Handler("block").ServeHTTP))
	http.HandleFunc("/api/v1/pprof/threadcreate", AdminAuth(a, pprof.Handler("threadcreate").ServeHTTP))
	http.HandleFunc("/api/v1/pprof/cmdline", AdminAuth(a, pprof.Cmdline))
	http.HandleFunc("/api/v1/pprof/profile", AdminAuth(a, pprof.Profile))
	http.HandleFunc("/api/v1/pprof/symbol", AdminAuth(a, pprof.Symbol))
	http.HandleFunc("/api/v1/pprof/trace", AdminAuth(a, pprof.Trace))
	http.HandleFunc("/api/v1/pprof/mutex", AdminAuth(a, pprof.Handler("mutex").ServeHTTP))

	http.HandleFunc("/api/v1/db", AdminAuth(a, func(w http.ResponseWriter, r *http.Request) {
		ReplyJSON(w, http.StatusOK, a.GetDB())
	}))

	http.HandleFunc("/api/v1/dbsave", AdminAuth(a, func(w http.ResponseWriter, r *http.Request) {
		saveDB(a)
		ReplyJSON(w, http.StatusOK, true)
	}))

	fullWebRootPath, _ := filepath.Abs(a.GetWebcoreConfig().WebRootDir)
	log.Println("Serving", fullWebRootPath, "on port", a.GetWebcoreConfig().Addr)

	fs := http.FileServer(http.Dir(fullWebRootPath))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			t := template.Must(template.ParseFiles(filepath.Join(a.GetWebcoreConfig().WebRootDir, "index.html")))
			data := struct {
				Config    *Config
				Appconfig interface{}
			}{a.GetWebcoreConfig(), a.GetAppConfig()}
			t.Execute(w, data)
		} else {
			fs.ServeHTTP(w, r)
		}
	})
}

func InitConfig(a App) {
	cfg := a.GetWebcoreConfig()
	cfg.Addr = "0.0.0.0:8001"
	cfg.WebRootDir = "frontend"
	cfg.Dir = "."
	cfg.Title = "My App"
	cfg.AdminUsers = []string{"github_cpq", "email_admin", "github_novlean"}
	if b, err := ioutil.ReadFile(*configFile); err == nil {
		if err2 := json.Unmarshal(b, cfg); err2 != nil {
			log.Fatalln("Error loading %s: %v", *configFile, err2)
		}
		json.Unmarshal(b, a.GetAppConfig()) // Best effort load of the app config
	}
}

func InitDB(a App) {
	dbFile := filepath.Join(a.GetWebcoreConfig().Dir, "db.json")
	content, err := ioutil.ReadFile(dbFile)
	if err == nil {
		if err2 := json.Unmarshal(content, a.GetDB()); err2 != nil {
			log.Fatalf("Error loading %s: %v", dbFile, err2)
		}
	}
}

func Run(a App) {
	InitConfig(a)
	InitDB(a)
	InitEndpoints(a)
	a.InitEndpoints()

	go func() {
		for {
			time.Sleep(time.Hour * 24)
			saveDB(a)
		}
	}()

	server := http.Server{Addr: a.GetWebcoreConfig().Addr}
	go server.ListenAndServe()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sigs
	saveDB(a)
}
