// +build !windows

package stm32

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

var (
	stlinkDevPrefixes = []string{"DIS_", "NODE_"}
)

func FindSTMMounts() ([]string, error) {
	dir := ""
	switch runtime.GOOS {
	case "linux":
		dir = filepath.Join("/", "media", os.Getenv("USER"))
	case "darwin":
		dir = "/Volumes"
	default:
		return nil, errors.Errorf("unsupported os %q", runtime.GOOS)
	}
	glog.V(1).Infof("Looking for ST-Link devices under %q", dir)
	ee, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to list %q", dir)
	}
	var res []string
	for _, e := range ee {
		for _, p := range stlinkDevPrefixes {
			if strings.HasPrefix(e.Name(), p) {
				n := filepath.Join(dir, e.Name())
				glog.V(1).Infof("  %q", n)
				res = append(res, n)
			}
		}
	}
	return res, nil
}
