// +build !windows

package main

import (
	"path/filepath"
	"runtime"
	"strings"
)

func enumerateSerialPorts() []string {
	if runtime.GOOS == "darwin" {
		list, _ := filepath.Glob("/dev/cu.*")
		filteredList := make([]string, 0)
		for _, s := range list {
			if !strings.Contains(s, "Bluetooth-") {
				filteredList = append(filteredList, s)
			}
		}
		return filteredList
	}
	list, _ := filepath.Glob("/dev/ttyUSB*")
	return list
}
