package devutil

import (
	"github.com/cesanta/errors"

	"github.com/mongoose-os/mos/mos/ourutil"
	"github.com/mongoose-os/mos/mos/flags"
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
