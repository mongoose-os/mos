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

/*
#include <direct.h>
#include <process.h>
#include <stdio.h>
#include <windows.h>

static char *s_canonical_path = "c:\\mos\\bin\\mos.exe";

void hideWindow(const char *prog) {
  ShowWindow(GetConsoleWindow(), SW_HIDE);
}
*/
import "C"

import (
	"os"

	zwebview "github.com/zserge/webview"
)

func osSpecificInit() {
	if startWebview && len(os.Args) == 1 {
		C.hideWindow(C.CString(os.Args[0]))
	}
}

func webview(url string) {
	zwebview.Open("mos tool", url, 1200, 600, true)
}
