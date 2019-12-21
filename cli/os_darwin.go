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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	zwebview "github.com/zserge/webview"
)

func enumerateSerialPorts() []string {
	list, _ := filepath.Glob("/dev/cu.*")
	var filteredList []string
	for _, s := range list {
		if !strings.Contains(s, "Bluetooth-") &&
			!strings.Contains(s, "-SPPDev") &&
			!strings.Contains(s, "-WirelessiAP") {
			filteredList = append(filteredList, s)
		}
	}
	sort.Strings(filteredList)
	return filteredList
}

func osSpecificInit() {
	// MacOS adds a unique UI process identifier flag when the executable
	// is started as an UI app. Remove it, as it confuses flags.
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "-psn_") {
		os.Args = os.Args[:1]

		// Add ourserlves to $PATH in order to make CLI work
		dirname, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		cmd := fmt.Sprintf(`grep %s ~/.profile || echo 'PATH=$PATH:%s' >> ~/.profile`, dirname, dirname)
		exec.Command("/bin/bash", "-c", cmd)
	}
}

func webview(url string) {
	zwebview.Open("mos tool", url, 1200, 600, true)
}
