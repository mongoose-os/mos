package devutil

import (
	"github.com/cesanta/errors"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/flags"
)

var defaultPort string

func GetPort() (string, error) {
	if *flags.Port != "auto" {
		return *flags.Port, nil
	}
	if defaultPort == "" {
		defaultPort = getDefaultPort()
		if defaultPort == "" {
			return "", errors.Errorf("--port not specified and none were found")
		}
		ourutil.Reportf("Using port %s", defaultPort)
	}
	return defaultPort, nil
}
