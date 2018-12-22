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
