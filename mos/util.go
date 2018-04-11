package main

import (
	"io"
	"time"

	"cesanta.com/common/go/ourutil"
)

func reportf(f string, args ...interface{}) {
	ourutil.Reportf(f, args...)
}

func freportf(logFile io.Writer, f string, args ...interface{}) {
	ourutil.Freportf(logFile, f, args...)
}

// If some command causes the device to reboot, the reboot actually happens
// after 100ms, so that the device is able to respond to the RPC request
// which causes the reboot.
//
// We shouldn't issue the next RPC request until the reboot happens, so
// waitForReboot should be called after each request which causes the reboot.
func waitForReboot() {
	time.Sleep(200 * time.Millisecond)
}
