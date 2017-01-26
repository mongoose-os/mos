package main

import (
	"os"
)

var defaultPort string

func getPort() string {
	if *portFlag != "auto" {
		return *portFlag
	}
	if defaultPort == "" {
		defaultPort = getDefaultPort()
		if defaultPort == "" {
			reportf("--port not specified and none were found")
			os.Exit(1)
		}
		reportf("Using port %s", defaultPort)
	}
	return defaultPort
}
