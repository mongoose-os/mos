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
	"fmt"
	"os/exec"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/fwbundle"
	"github.com/mongoose-os/mos/cli/ourutil"
)

func checkSTFlashPath(path string) string {
	if path == "" {
		return ""
	}
	var err error
	path, err = exec.LookPath(path)
	if err != nil {
		return ""
	}
	return path
}

func flashSTFlash(fw *fwbundle.FirmwareBundle, opts *FlashOpts) error {
	stFlashPath := checkSTFlashPath(opts.STFlashPath)
	if stFlashPath == "" {
		return fmt.Errorf("st-flash utility not found")
	}
	if opts.KeepFS {
		return errors.Errorf("--keep-fs is not supportef for STM32")
	}
	ourutil.Reportf("Using %s", stFlashPath)
	for _, p := range fw.PartsByAddr() {
		fname, dataLen, err := fw.GetPartDataFile(p.Name)
		if err != nil {
			return errors.Trace(err)
		}
		ourutil.Reportf("Flashing %q (%d @ %#x)...", p.Name, dataLen, p.Addr)
		cmd := []string{stFlashPath}
		if opts.Serial != "" {
			cmd = append(cmd, "--serial", fmt.Sprintf("0x%s", opts.Serial))
		}
		cmd = append(cmd, "write", fname, fmt.Sprintf("%#x", p.Addr))
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, cmd...); err != nil {
			return errors.Annotatef(err, "st-flash command failed")
		}
	}
	return nil
}
