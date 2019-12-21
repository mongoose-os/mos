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
package stm32

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/fwbundle"
	"github.com/mongoose-os/mos/cli/flash/common"
	"github.com/mongoose-os/mos/cli/ourutil"
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

func flashShare(fw *fwbundle.FirmwareBundle, opts *FlashOpts) error {
	if opts.KeepFS {
		// It's not easy: fs is included in the big blob.
		// We'd need to read the fs first to preserve it.
		return errors.Errorf("--keep-fs is not supportef for STM32")
	}

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
