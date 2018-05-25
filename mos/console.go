package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"cesanta.com/mos/debug_core_dump"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/timestamp"

	"github.com/cesanta/errors"
	"github.com/cesanta/go-serial/serial"
	flag "github.com/spf13/pflag"
)

// console specific flags
var (
	baudRate        uint
	noInput         bool
	hwFC            bool
	setControlLines bool
	tsfSpec         string
	catchCoreDumps  bool
)

var (
	tsFormat string
)

func init() {
	flag.UintVar(&baudRate, "baud-rate", 115200, "Serial port speed")
	flag.BoolVar(&noInput, "no-input", false,
		"Do not read from stdin, only print device's output to stdout")
	flag.BoolVar(&hwFC, "hw-flow-control", false, "Enable hardware flow control (CTS/RTS)")
	flag.BoolVar(&setControlLines, "set-control-lines", true, "Set RTS and DTR explicitly when in console/RPC mode")
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
	tf, err := ioutil.TempFile("", fmt.Sprintf("core-%s-%s-%s", info.App, info.Platform, time.Now().Format("20060102-150405.")))
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

func console(ctx context.Context, devConn *dev.DevConn) error {
	in, out := os.Stdin, os.Stdout
	port, err := getPort()
	if err != nil {
		return errors.Trace(err)
	}

	s, err := serial.Open(serial.OpenOptions{
		PortName:            port,
		BaudRate:            baudRate,
		HardwareFlowControl: hwFC,
		DataBits:            8,
		ParityMode:          serial.PARITY_NONE,
		StopBits:            1,
		MinimumReadSize:     1,
	})
	if err != nil {
		return errors.Annotatef(err, "failed to open %s", port)
	}

	if setControlLines || *invertedControlLines {
		bFalse := *invertedControlLines
		s.SetDTR(bFalse)
		s.SetRTS(bFalse)
	}

	cctx, cancel := context.WithCancel(ctx)
	go func() { // Serial -> Stdout
		var curLine []byte
		var coreDump []byte
		coreDumping := false
		lastCDProgress := 0

		for {
			buf := make([]byte, 100)
			n, err := s.Read(buf)
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
				cont := len(curLine) > 0
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
			}
			if !coreDumping && len(buf) > 0 {
				printConsoleLine(out, len(curLine) == 0, buf)
			}
			curLine = append(curLine, buf...)
		}
	}()
	go func() { // Stdin -> Serial
		// If no input, just block forever
		if noInput {
			select {}
		}
		for {
			buf := make([]byte, 1)
			n, err := in.Read(buf)
			if n > 0 {
				s.Write(buf[:n])
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()
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
