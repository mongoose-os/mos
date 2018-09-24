package stm32

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"cesanta.com/mos/flash/common"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
)

var (
	stlinkDevPrefixes = []string{"DIS_", "NODE_"}
)

type FlashOpts struct {
	ShareName string
	Timeout   time.Duration
}

func Flash(fw *common.FirmwareBundle, opts *FlashOpts) error {
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
