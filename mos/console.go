package main

import (
	"bytes"
	"context"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"cesanta.com/common/go/mgrpc"
	"cesanta.com/common/go/mgrpc/codec"
	"cesanta.com/common/go/mgrpc/frame"
	"cesanta.com/common/go/ourjson"
	"cesanta.com/mos/debug_core_dump"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/devutil"
	"cesanta.com/mos/flags"
	"cesanta.com/mos/timestamp"

	"github.com/cesanta/errors"
	"github.com/cesanta/go-serial/serial"
	flag "github.com/spf13/pflag"
)

// console specific flags
var (
	baudRateFlag    uint
	noInput         bool
	hwFCFlag        bool
	setControlLines bool
	tsfSpec         string
	catchCoreDumps  bool
)

var (
	tsFormat string
)

func init() {
	flag.BoolVar(&noInput, "no-input", false,
		"Do not read from stdin, only print device's output to stdout")
	flag.BoolVar(&catchCoreDumps, "catch-core-dumps", true, "Catch and save core dumps")

	flag.StringVar(&tsfSpec, "timestamp", "StampMilli",
		"Prepend each line with a timestamp in the specified format. A number of specifications are supported:"+
			"simple 'yes' or 'true' will use UNIX Epoch + .microseconds; the Go way of specifying date/time "+
			"format, as described in https://golang.org/pkg/time/, including the constants "+
			"(so --timestamp=UnixDate will work, as will --timestamp=Stamp); the strftime(3) format "+
			"(see http://strftime.org/)")

	flag.Lookup("timestamp").NoOptDefVal = "true" // support just passing --timestamp

	for _, f := range []string{"no-input", "timestamp"} {
		hiddenFlags = append(hiddenFlags, f)
	}
}

func consoleInit() {
	if tsfSpec != "" {
		tsFormat = timestamp.ParseTimeStampFormatSpec(tsfSpec)
	}
}

func FormatTimestampNow() string {
	ts := ""
	if tsFormat != "" {
		ts = fmt.Sprintf("[%s] ", timestamp.FormatTimestamp(time.Now(), tsFormat))
	}
	return ts
}

func printConsoleLine(out io.Writer, addTS bool, line []byte) {
	if tsfSpec != "" && addTS {
		fmt.Printf("%s", FormatTimestampNow())
	}
	removeNonText(line)
	out.Write(line)
}

func analyzeCoreDump(out io.Writer, cd []byte) error {
	info, err := debug_core_dump.GetInfoFromCoreDump(cd)
	cwd, _ := os.Getwd() // Mac docker cannot mount dirs from /tmp. Thus, create core in the CWD
	tf, err := ioutil.TempFile(cwd, fmt.Sprintf("core-%s-%s-%s", info.App, info.Platform, time.Now().Format("20060102-150405.")))
	if err != nil {
		return errors.Annotatef(err, "failed open core dump file")
	}
	tfn := tf.Name()
	tf.Write([]byte(debug_core_dump.CoreDumpStart))
	tf.Write([]byte("\r\n"))
	printConsoleLine(out, true, []byte(fmt.Sprintf("mos: wrote to %s (%d bytes)\n", tfn, len(cd))))
	if _, err := tf.Write(cd); err != nil {
		tf.Close()
		return errors.Annotatef(err, "failed to write core dump to %s", tfn)
	}
	tf.Write([]byte("\r\n"))
	tf.Write([]byte(debug_core_dump.CoreDumpEnd))
	tf.Close()
	printConsoleLine(out, true, []byte("mos: analyzing core dump\n"))
	return debug_core_dump.DebugCoreDumpF(tfn, "", true)
}

type chanReader struct {
	rch   chan []byte
	rdata []byte
}

func (chr *chanReader) Read(buf []byte) (int, error) {
	if chr.rch == nil {
		return 0, io.EOF
	}
	if len(chr.rdata) == 0 {
		rdata, ok := <-chr.rch
		if !ok {
			return 0, io.EOF
		}
		chr.rdata = rdata
	}
	b := bytes.NewBuffer(chr.rdata)
	n, err := b.Read(buf)
	if err == nil {
		chr.rdata = chr.rdata[n:]
	}
	return n, err
}

func (chr *chanReader) Write(buf []byte) (int, error) {
	return 0, io.EOF
}

func (chr *chanReader) Close() error {
	rch := chr.rch
	chr.rch = nil
	if rch != nil {
		close(rch)
	}
	return nil
}

func console(ctx context.Context, devConn dev.DevConn) error {

	var r io.Reader
	var w io.Writer

	purl, err := url.Parse(*flags.Port)
	switch {
	case err == nil && (purl.Scheme == "mqtt" || purl.Scheme == "mqtts"):
		chr := &chanReader{rch: make(chan []byte)}
		opts, topic, err := codec.MQTTClientOptsFromURL(*flags.Port, "", "", "")
		if err != nil {
			return errors.Errorf("invalid MQTT port URL format")
		}
		tlsConfig, err := flags.TLSConfigFromFlags()
		if err != nil {
			return errors.Annotatef(err, "inavlid TLS config")
		}
		opts.SetTLSConfig(tlsConfig)
		opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
			reportf("MQTT connection closed")
			chr.Close()
		})
		cli := mqtt.NewClient(opts)
		reportf("Connecting to %s...", opts.Servers[0])
		token := cli.Connect()
		token.Wait()
		if err := token.Error(); err != nil {
			return errors.Annotatef(err, "MQTT connect error")
		}
		topic += "/log"
		token = cli.Subscribe(topic, 0 /* qos */, func(c mqtt.Client, m mqtt.Message) {
			chr.rch <- m.Payload()
		})
		token.Wait()
		if err := token.Error(); err != nil {
			return errors.Annotatef(err, "MQTT subscribe error")
		}
		reportf("Subscribed to %s", topic)
		r = chr

	case err == nil && purl.Scheme == "udp":
		hpp := strings.Split(purl.Host, ":")
		if len(hpp) != 2 {
			return errors.Errorf("invalid UDP port URL format, must be udp://:port/ or udp://ip:port/ %q %d", purl.Host, len(hpp))
		}
		p, err := strconv.Atoi(hpp[1])
		if err != nil {
			return errors.Errorf("invalid UDP port format, must be udp://:port/ or udp://ip:port/")
		}
		addr := net.UDPAddr{
			IP:   net.ParseIP(hpp[0]),
			Port: p,
		}
		udpc, err := net.ListenUDP("udp", &addr)
		if err != nil {
			return errors.Annotatef(err, "failed to open listner at %+v", addr)
		}
		if addr.IP != nil {
			reportf("Listening on UDP %s:%d...", addr.IP, addr.Port)
		} else {
			reportf("Listening on UDP port %d...", addr.Port)
		}
		defer udpc.Close()
		r, w = udpc, udpc

	case err == nil && (purl.Scheme == "ws" || purl.Scheme == "wss"):
		// Connect to mDash and activate event forwarding.
		chr := &chanReader{rch: make(chan []byte)}
		devConn, err = devutil.CreateDevConnFromFlags(ctx)
		if err != nil {
			return errors.Trace(err)
		}
		if err = devConn.Call(ctx, "Dash.Console.Subscribe", nil, nil); err != nil {
			return errors.Trace(err)
		}
		devConn.(*dev.MosDevConn).RPC.AddHandler("Dash.Console.Event", func(c mgrpc.MgRPC, f *frame.Frame) *frame.Frame {
			var ev struct {
				DevId string             `json:"id"`
				Name  string             `json:"name"`
				Data  ourjson.RawMessage `json:"data"`
			}
			f.Params.UnmarshalInto(&ev)
			var s string
			if ev.Name == "rpc.out.Log" {
				var logEv struct {
					Timestamp float64 `json:"t"`
					FD        int     `json:"fd"`
					Seq       int     `json:"seq"`
					Data      string  `json:"data"`
				}
				ev.Data.UnmarshalInto(&logEv)
				s = fmt.Sprintf("%d %.3f %d|%s", logEv.Seq, logEv.Timestamp, logEv.FD, logEv.Data)
			} else {
				b, _ := ev.Data.MarshalJSON()
				s = fmt.Sprintf("%s %s\n", ev.Name, string(b))
			}
			chr.rch <- []byte(s)
			return nil
		})
		r = chr

	default:
		// Everything else is treated as a serial port.
		port, err := devutil.GetPort()
		if err != nil {
			return errors.Trace(err)
		}

		sp, err := serial.Open(serial.OpenOptions{
			PortName:            port,
			BaudRate:            uint(*flags.BaudRate),
			HardwareFlowControl: *flags.HWFC,
			DataBits:            8,
			ParityMode:          serial.PARITY_NONE,
			StopBits:            1,
			MinimumReadSize:     1,
		})
		if err != nil {
			return errors.Annotatef(err, "failed to open %s", port)
		}
		if *flags.SetControlLines || *flags.InvertedControlLines {
			bFalse := *flags.InvertedControlLines
			sp.SetDTR(bFalse)
			sp.SetRTS(bFalse)
		}
		defer sp.Close()
		r, w = sp, sp
	}

	return consoleReadWrite(ctx, r, w)
}

func consoleReadWrite(ctx context.Context, r io.Reader, w io.Writer) error {
	in, out := os.Stdin, os.Stdout
	cctx, cancel := context.WithCancel(ctx)
	go func() { // Serial -> Stdout
		var curLine []byte
		var coreDump []byte
		coreDumping := false
		lastCDProgress := 0
		cont := false

		for {
			buf := make([]byte, 1500)
			n, err := r.Read(buf)
			if err != nil {
				reportf("read err %s", err)
				cancel()
				return
			}
			if n <= 0 {
				continue
			}
			buf = buf[:n]
			for {
				lf := bytes.IndexAny(buf, "\n")
				if lf < 0 {
					break
				}
				chunk := buf[:lf+1]
				curLine = append(curLine, chunk...)
				if catchCoreDumps {
					tsl := bytes.TrimSpace(curLine)
					if !coreDumping && bytes.Compare(tsl, []byte(debug_core_dump.CoreDumpStart)) == 0 {
						printConsoleLine(out, !cont, chunk)
						printConsoleLine(out, true, []byte("mos: catching core dump\n"))
						coreDumping = true
						coreDump = nil
					} else if coreDumping {
						if bytes.Compare(tsl, []byte(debug_core_dump.CoreDumpEnd)) == 0 {
							if lastCDProgress > 0 {
								printConsoleLine(out, false, []byte("\n"))
							}
							printConsoleLine(out, true, curLine)
							coreDumping = false
							lastCDProgress = 0
							curLine = nil
							if err := analyzeCoreDump(out, coreDump); err != nil {
								printConsoleLine(out, true, []byte(fmt.Sprintf("mos: %s\n", err)))
							}
						} else {
							// There should be no empty lines in the CD body.
							// If we encounter an empty line, this means device rebooted without finishing the CD.
							if len(tsl) == 0 {
								printConsoleLine(out, true, []byte("mos: core dump aborted\n"))
								coreDumping = false
								lastCDProgress = 0
								coreDump = nil
							} else {
								coreDump = append(coreDump, curLine...)
								if len(coreDump) > lastCDProgress+32*1024 {
									printConsoleLine(out, lastCDProgress == 0, []byte("."))
									lastCDProgress = len(coreDump)
								}
							}
						}
					}
				}
				if !coreDumping && curLine != nil {
					printConsoleLine(out, !cont, chunk)
				}
				curLine = nil
				buf = buf[lf+1:]
				cont = false
			}
			if !coreDumping && len(buf) > 0 {
				printConsoleLine(out, !cont, buf)
				cont = true
			}
			curLine = append(curLine, buf...)
		}
	}()
	if w != nil && !noInput {
		go func() { // Stdin -> Serial
			for {
				buf := make([]byte, 1)
				n, err := in.Read(buf)
				if n > 0 {
					w.Write(buf[:n])
				}
				if err != nil {
					cancel()
					return
				}
			}
		}()
	}
	<-cctx.Done()
	return nil
}

func removeNonText(data []byte) {
	for i, c := range data {
		if (c < 0x20 && c != 0x0a && c != 0x0d && c != 0x1b /* Esc */) || c >= 0x80 {
			data[i] = 0x20
		}
	}
}
