package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/cesanta/errors"
	"github.com/cesanta/go-serial/serial"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"
	"github.com/skratchdot/open-golang/open"

	"golang.org/x/net/websocket"
)

var (
	httpPort    = 1992
	wsClients   = make(map[*websocket.Conn]int)
	consoleChan = make(chan bool)
	consoleWg   sync.WaitGroup
	wwwRoot     = ""
)

type wsmessage struct {
	Cmd  string `json:"cmd"`
	Data string `json:"data"`
}

func wsSend(ws *websocket.Conn, m wsmessage) {
	t, _ := json.Marshal(m)
	websocket.Message.Send(ws, string(t))
}

func wsBroadcast(m wsmessage) {
	for ws := range wsClients {
		wsSend(ws, m)
	}
}

type errmessage struct {
	Error string `json:"error"`
}

func wsHandler(ws *websocket.Conn) {
	defer func() {
		delete(wsClients, ws)
		ws.Close()
	}()
	wsClients[ws] = 1

	for {
		var text string
		err := websocket.Message.Receive(ws, &text)
		if err != nil {
			glog.Infof("Websocket recv error: %v, closing connection", err)
			return
		}
	}
}

func openSerial(port string) (serial.Serial, error) {
	glog.Infof("opening %s...", port)
	s, err := serial.Open(serial.OpenOptions{
		PortName:            port,
		BaudRate:            115200,
		HardwareFlowControl: false,
		DataBits:            8,
		ParityMode:          serial.PARITY_NONE,
		StopBits:            1,
		MinimumReadSize:     1,
	})
	glog.Infof("opened console: %v %v", s, err)
	if err != nil {
		glog.Errorf("failed to open %s: %v", port, err)
		return nil, errors.Trace(err)
	}
	s.SetDTR(false)
	s.SetRTS(false)
	return s, err
}

func reportSerialPorts() {
	for {
		list := enumerateSerialPorts()
		wsBroadcast(wsmessage{"ports", strings.Join(list, ",")})
		time.Sleep(time.Second)
	}
}

func interruptConsole() {
	consoleWg.Add(1)
	consoleChan <- true
}

func reportConsoleLogs() {
	for {
		glog.Infof("openSerial: waiting for commands to finish...")
		consoleWg.Wait()
		s, err := openSerial(*port)
		if err != nil {
			time.Sleep(time.Millisecond * 200)
			select {
			case <-consoleChan:
			default:
			}
			continue
		}

	readLoop:
		for {
			buf := make([]byte, 100)
			n, err := s.Read(buf)
			if n > 0 {
				removeJunk(buf[:n])
				wsBroadcast(wsmessage{"console", string(buf[:n])})
			}
			if err != nil {
				glog.Errorf("Error reading from %s: %v", *port, err)
				break
			}

			select {
			case <-consoleChan:
				glog.Infof("INTERRUPT!!")
				break readLoop
			default:
			}
		}
		glog.Infof("closing console")
		s.Close()
	}
}

func httpReply(w http.ResponseWriter, result interface{}, err error) {
	var msg []byte
	if err != nil {
		msg, _ = json.Marshal(errmessage{err.Error()})
	} else {
		s, ok := result.(string)
		if ok && isJSON(s) {
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

func init() {
	flag.StringVar(&wwwRoot, "web-root", "", "UI Web root to use instead of built-in")
	hiddenFlags = append(hiddenFlags, "web-root")
}

func startUI() {

	glog.CopyStandardLogTo("INFO")
	go reportSerialPorts()
	go reportConsoleLogs()
	http.Handle("/ws", websocket.Handler(wsHandler))

	http.HandleFunc("/flash", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		r.ParseForm()
		*firmware = r.FormValue("firmware")

		interruptConsole()
		defer consoleWg.Done()

		err := flash()
		httpReply(w, true, err)
	})

	http.HandleFunc("/wifi", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		args := []string{
			"wifi.ap.enable=false",
			"wifi.sta.enable=true",
			fmt.Sprintf("wifi.sta.ssid=%s", r.FormValue("ssid")),
			fmt.Sprintf("wifi.sta.pass=%s", r.FormValue("pass")),
		}

		interruptConsole()
		defer consoleWg.Done()

		err := internalConfigSet(args)
		result := "false"
		if err == nil {
			for {
				time.Sleep(time.Millisecond * 500)
				res2, _ := callDeviceService("Config.GetNetworkStatus", "")
				if res2 != "" {
					type Netcfg struct {
						Wifi struct {
							Ssid   string `json:"ssid"`
							Sta_ip string `json:"sta_ip"`
							Status string `json:"status"`
						} `json:"wifi"`
					}
					var c Netcfg
					yaml.Unmarshal([]byte(res2), &c)
					if c.Wifi.Status == "got ip" {
						result = fmt.Sprintf("\"%s\"", c.Wifi.Sta_ip)
						break
					} else if c.Wifi.Status == "connecting" || c.Wifi.Status == "" || c.Wifi.Status == "associated" {
						// Still connecting, wait
					} else {
						err = errors.Errorf("%s", c.Wifi.Status)
						break
					}
				}
			}
		}
		httpReply(w, result, err)
	})

	http.HandleFunc("/policies", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		awsRegion = r.FormValue("region")
		arr, err := getAWSIoTPolicyNames()
		httpReply(w, arr, err)
	})

	http.HandleFunc("/regions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		httpReply(w, getAWSRegions(), nil)
	})

	http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		interruptConsole()
		defer consoleWg.Done()

		// Open a port, and close it immediately. Report success.
		*port = r.FormValue("port")
		s, err := openSerial(*port)
		if err == nil {
			s.Close()
		}
		httpReply(w, true, err)
	})

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		httpReply(w, Version, nil)
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		interruptConsole()
		defer consoleWg.Done()

		text, err := getFile(r.FormValue("name"))
		if err == nil {
			text2, err2 := json.Marshal(text)
			if err2 == nil {
				text = string(text2)
			} else {
				err = err2
			}
		}
		httpReply(w, text, err)
	})

	http.HandleFunc("/aws-iot-setup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		interruptConsole()
		defer consoleWg.Done()

		awsIoTPolicy = r.FormValue("policy")
		awsRegion = r.FormValue("region")
		err := awsIoTSetup()
		httpReply(w, true, err)
	})

	http.HandleFunc("/aws-store-creds", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := storeCreds(r.FormValue("key"), r.FormValue("secret"))
		httpReply(w, true, err)
	})

	http.HandleFunc("/setenv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		myPort := r.FormValue("port")
		if myPort != "" {
			*port = myPort
		}
		httpReply(w, true, nil)
	})

	http.HandleFunc("/check-aws-credentials", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := checkAwsCredentials()
		httpReply(w, err == nil, nil)
	})

	http.HandleFunc("/infolog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		glog.Flush()
		pattern := fmt.Sprintf("%s/mos*INFO*.%d", os.TempDir(), os.Getpid())
		paths, err := filepath.Glob(pattern)
		if err == nil && len(paths) > 0 {
			http.ServeFile(w, r, paths[0])
		}
	})

	http.HandleFunc("/call", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		method := r.FormValue("method")

		interruptConsole()
		defer consoleWg.Done()

		if method == "" {
			httpReply(w, nil, errors.Errorf("Expecting method"))
			return
		}
		args := r.FormValue("args")
		result, err := callDeviceService(method, args)
		httpReply(w, result, err)
	})

	if wwwRoot != "" {
		http.Handle("/", http.FileServer(http.Dir(wwwRoot)))
	} else {
		assetInfo := func(path string) (os.FileInfo, error) {
			return os.Stat(path)
		}
		http.Handle("/", http.FileServer(&assetfs.AssetFS{Asset: Asset,
			AssetDir: AssetDir, AssetInfo: assetInfo, Prefix: "web_root"}))
	}
	addr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("Web UI started. Point your browser at %s\n", url)
	fmt.Printf("For advanced functionality, start mgos from the command line: mgos --ui=false\n")
	open.Start(url)
	log.Fatal(http.ListenAndServe(addr, nil))
}
