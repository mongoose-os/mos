package stm32

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cesanta.com/common/go/fwbundle"
	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/flash/common"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

var (
	stlinkDevPrefixes = []string{"DIS_", "NODE_"}
)

func getSTLinkMountsInDir(dir string) ([]string, error) {
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
				ourutil.Reportf("Found STLink mount: %s", n)
				res = append(res, n)
			}
		}
	}
	return res, nil
}

type FlashOpts struct {
	ShareName string
	Timeout   time.Duration
}

func Flash(fw *fwbundle.FirmwareBundle, opts *FlashOpts) error {
	data, err := fw.GetPartData("app")
	if err != nil {
		return errors.Annotatef(err, "invalid manifest")
	}

	name := filepath.Join(opts.ShareName, fw.Parts["app"].Src)

	common.Reportf("Copying %s to %s...", fw.Parts["app"].Src, opts.ShareName)
	err = ioutil.WriteFile(name, data, 0)
	if err != nil {
		return errors.Trace(err)
	}

	common.Reportf("Waiting for operation to complete...")

	start := time.Now()

	for {
		_, err = os.Stat(name)
		if err != nil {
			if os.IsNotExist(err) {
				// File is disappeared: operation ok
				return nil
			} else {
				// On Windows, this sometimes raises spurious errors, like CreateFile error.
				// In fact, flashing went just fine.
				glog.Infof("Error during Stat (most likely benign): %s", err)
				return nil
			}
		}

		if time.Since(start) > opts.Timeout {
			return errors.Errorf("timeout")
		}

		time.Sleep(1000 * time.Millisecond)
	}
}
