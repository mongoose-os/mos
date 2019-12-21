//
// Copyright (c) 2014-2019 Cesanta Software Limited
// All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
package main

import (
	"io"
	"time"

	"github.com/mongoose-os/mos/cli/ourutil"
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
