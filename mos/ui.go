package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	usr "os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/devutil"
	"cesanta.com/mos/flags"
	"cesanta.com/mos/version"
	"github.com/cesanta/errors"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"
	shellwords "github.com/mattn/go-shellwords"
	"github.com/skratchdot/open-golang/open"
	flag "github.com/spf13/pflag"
	"golang.org/x/net/websocket"
)

var (
	httpAddr     = "localhost:1992"
	wwwRoot      = ""
	startBrowser = true
	startWebview = true
	wsClients    = make(map[*websocket.Conn]int)
	wsClientsMtx = sync.Mutex{}
	lockChan     = make(chan int)
	unlockChan   = make(chan bool)
)

type wsmessage struct {
	Cmd  string `json:"cmd"`
	Data string `json:"data"`
}

type errmessage struct {
	Error string `json:"error"`
}

func httpReplyExt(w http.ResponseWriter, result interface{}, err error, asJSON bool) {
	var msg []byte
	if err != nil {
		msg, _ = json.Marshal(errmessage{err.Error()})
	} else {
		if asJSON {
			s := result.(string)
			msg = []byte(fmt.Sprintf(`{"result": %s}`, s))
		} else {
			r := map[string]interface{}{
				"result": result,
			}
			msg, err = json.Marshal(r)
		}
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, string(msg))
	} else {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(msg))
	}
}

func httpReply(w http.ResponseWriter, result interface{}, err error) {
	s, ok := result.(string)
	asJSON := ok && isJSON(s)
	httpReplyExt(w, result, err, asJSON)
}

func wsSend(ws *websocket.Conn, m wsmessage) {
	t, _ := json.Marshal(m)
	websocket.Message.Send(ws, string(t))
}

func wsBroadcast(m wsmessage) {
	wsClientsMtx.Lock()
	defer wsClientsMtx.Unlock()
	for ws := range wsClients {
		wsSend(ws, m)
	}
}

func wsHandler(ws *websocket.Conn) {
	defer func() {
		wsClientsMtx.Lock()
		defer wsClientsMtx.Unlock()
		delete(wsClients, ws)
		ws.Close()
	}()
	wsClientsMtx.Lock()
	wsClients[ws] = 1
	wsClientsMtx.Unlock()

	for {
		var text string
		err := websocket.Message.Receive(ws, &text)
		if err != nil {
			glog.Infof("Websocket recv error: %v, closing connection", err)
			return
		}
	}
}

func init() {
	flag.StringVar(&wwwRoot, "web-root", "", "UI Web root to use instead of built-in")
	hiddenFlags = append(hiddenFlags, "web-root")

	flag.StringVar(&httpAddr, "http-addr", "127.0.0.1:1992", "Web UI HTTP address")
	hiddenFlags = append(hiddenFlags, "http-addr")

	flag.BoolVar(&startBrowser, "start-browser", true, "Automatically start browser")
	hiddenFlags = append(hiddenFlags, "start-browser")

	flag.BoolVar(&startWebview, "start-webview", startWebview, "Automatically start WebView")
	hiddenFlags = append(hiddenFlags, "start-webview")
}

type wsWriter struct{}

func (w *wsWriter) Write(p []byte) (int, error) {
	wsBroadcast(wsmessage{"uart", string(p)})
	return len(p), nil
}

func startUI(ctx context.Context, devConn dev.DevConn) error {
	fullMosPath, _ := os.Executable()
	fullWebRootPath, _ := filepath.Abs(wwwRoot)

	// Redirect stdio to websocket
	origStdout, origStderr := os.Stdout, os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout, os.Stderr = w, w
	go func() {
		for {
			data := make([]byte, 512)
			n, err := r.Read(data)
			if err != nil {
				break
			}
			wsBroadcast(wsmessage{"stdio", string(data[:n])})
		}
	}()

	// Run `mos console` when idle and port is chosen
	go func() {
		numActiveMosCommands := 0 // mos commands that want serial
		var cancel context.CancelFunc
		for {
			n := <-lockChan
			if numActiveMosCommands == 0 && cancel != nil {
				cancel()
			}
			if n > 0 {
				numActiveMosCommands++
				unlockChan <- true
			} else if n < 0 {
				numActiveMosCommands--
			}
			if numActiveMosCommands == 0 {
				if *flags.Port == "" {
					cancel = nil
				} else {
					var ctx2 context.Context
					ctx2, cancel = context.WithCancel(ctx)
					cmd := exec.CommandContext(ctx2, fullMosPath, "console", "--port", *flags.Port)
					w := wsWriter{}
					cmd.Stdout = &w
					cmd.Stderr = &w
					cmd.Stdin = os.Stdin // This makes `mos console` process close when we exit
					cmd.Start()
				}
			}
		}
	}()

	if *flags.Port != "" {
		lockChan <- 0 // Start mos console if port is set
	}

	http.HandleFunc("/version-tag", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		httpReplyExt(w, version.GetMosVersion(), nil, false /* not as JSON */)
	})

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		httpReply(w, version.BuildId, nil)
	})

	http.HandleFunc("/sysinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		u, _ := usr.Current()
		info := struct {
			OS      string `json:"os"`
			Home    string `json:"home"`
			Version string `json:"version"`
		}{runtime.GOOS, u.HomeDir, version.BuildId}
		httpReply(w, &info, nil)
	})

	http.HandleFunc("/getports", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		type GetPortsResult struct {
			PortFlag string
			Ports    []string
		}
		reply := GetPortsResult{*flags.Port, devutil.EnumerateSerialPorts()}
		httpReply(w, reply, nil)
	})

	http.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		cmd := r.FormValue("cmd")
		w.Header().Set("Content-Type", "application/json")
		err := open.Start(cmd)
		httpReply(w, true, err)
	})

	http.HandleFunc("/serial", func(w http.ResponseWriter, r *http.Request) {
		port := r.FormValue("port")
		if port != *flags.Port {
			*flags.Port = port
			lockChan <- 0 // let console goroutine know the port has changed
		}
		httpReply(w, true, nil)
	})

	http.HandleFunc("/terminal", func(w http.ResponseWriter, r *http.Request) {
		str := r.FormValue("cmd")
		if str == "" {
			httpReply(w, false, fmt.Errorf("empty command"))
			return
		}
		args, err := shellwords.Parse(str)
		if err != nil {
			httpReply(w, true, err)
			return
		}
		if len(args) > 0 && args[0] == "mos" {
			args[0] = fullMosPath
			if len(args) < 2 {
				httpReply(w, true, fmt.Errorf("Command missing"))
				return
			}
			cmd := getCommand(args[1])
			if cmd == nil {
				httpReply(w, true, fmt.Errorf("Unknown command"))
				return
			}

			// Release port if mos command wants to grab it
			if cmd.needDevConn != No {
				if *flags.Port == "" {
					httpReply(w, true, fmt.Errorf("Port not chosen"))
					return
				}
				args = append(args, "--port")
				args = append(args, *flags.Port)
				lockChan <- 1
				defer func() {
					lockChan <- -1
				}()
				<-unlockChan
			}
		}

		if len(args) > 0 && args[0] == "cd" {
			dir := filepath.Dir(fullMosPath)
			if len(args) > 1 {
				dir = args[1]
			}
			err := os.Chdir(dir)
			cwd, _ := os.Getwd()
			httpReply(w, cwd, err)
		} else {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			httpReply(w, false, err)
		}
	})

	http.Handle("/ws", websocket.Handler(wsHandler))

	// Observe port changes
	go func() {
		initialised := false
		prevList := ""
		for {
			ports := devutil.EnumerateSerialPorts()
			sort.Strings(ports)
			list := strings.Join(ports, ",")
			if initialised && list != prevList {
				wsBroadcast(wsmessage{"portchange", list})
			}
			prevList = list
			time.Sleep(time.Second)
			initialised = true
		}
	}()

	if wwwRoot != "" {
		http.HandleFunc("/", addNoCacheHeader(http.FileServer(http.Dir(fullWebRootPath))))
	} else {
		assetInfo := func(path string) (os.FileInfo, error) {
			return os.Stat(path)
		}
		http.Handle("/", addNoCacheHeader(http.FileServer(&assetfs.AssetFS{Asset: Asset,
			AssetDir: AssetDir, AssetInfo: assetInfo, Prefix: "web_root"})))
	}
	url := fmt.Sprintf("http://%s", httpAddr)

	ourutil.Reportf("To get a list of available commands, start with --help")
	listener, err := net.Listen("tcp", httpAddr)
	if err != nil {
		os.Stdout, os.Stderr = origStdout, origStderr
		return errors.Trace(err)
	}
	if startWebview && runtime.GOOS != "linux" {
		ourutil.Reportf("Starting Web UI in a webview..")
		go http.Serve(listener, nil)
		webview(url)
	} else {
		ourutil.Reportf("Starting Web UI. If the browser does not start, navigate to %s", url)
		if startBrowser {
			open.Start(url)
		}
		if err := http.Serve(listener, nil); err != nil {
			os.Stdout, os.Stderr = origStdout, origStderr
			return errors.Trace(err)
		}
	}

	// Unreacahble
	return nil
}

func addNoCacheHeader(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		handler.ServeHTTP(w, r)
	}
}
