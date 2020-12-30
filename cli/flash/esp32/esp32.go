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
package esp32

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/flash/esp"
)

var (
	FlashSizeToId = map[string]int{
		// +1, to distinguish from null-value
		"8m":   1,
		"16m":  2,
		"32m":  3,
		"64m":  4,
		"128m": 5,
	}

	FlashSizes = map[int]int{
		0: 1048576,
		1: 2097152,
		2: 4194304,
		3: 8388608,
		4: 16777216,
	}
)

func GetChipDescr(rrw esp.RegReaderWriter) (string, error) {
	_, _, fusesByName, err := ReadFuses(rrw)
	if err != nil {
		return "", errors.Annotatef(err, "failed to read eFuses")
	}
	cpkg02, err := fusesByName["chip_pkg02"].Value(false)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get chip_pkg02")
	}
	cpkg3, err := fusesByName["chip_pkg3"].Value(false)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get chip_pkg3")
	}
	disable_app_cpu, err := fusesByName["disable_app_cpu"].Value(false)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get disable_app_cpu")
	}
	cpkg := (cpkg3.Uint64() << 3) | cpkg02.Uint64()
	single_core := (disable_app_cpu.Uint64() == 1)

	crev1, err := fusesByName["chip_rev1"].Value(false)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get chip_rev0")
	}
	crev2, err := fusesByName["chip_rev2"].Value(false)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get chip_rev1")
	}
	apb_ctl_date, err := rrw.ReadReg(0x3ff6607c)
	if err != nil {
		return "", errors.Annotatef(err, "failed to read apb_ctl_date")
	}
	chip_rev := 0
	if crev1.Uint64() != 0 {
		chip_rev++
		if crev2.Uint64() != 0 {
			chip_rev++
			if (apb_ctl_date & (1 << 31)) != 0 {
				chip_rev++
			}
		}
	}

	chip_pkg := ""
	switch cpkg {
	case 0:
		if single_core {
			chip_pkg = "ESP32-S0WDQ6"
		} else {
			chip_pkg = "ESP32D0WDQ6"
			if chip_rev == 3 {
				chip_pkg += "-V3"
			}
		}
	case 1:
		if single_core {
			chip_pkg = "ESP32S0WD"
		} else {
			chip_pkg = "ESP32D0WD"
			if chip_rev == 3 {
				chip_pkg += "-V3"
			}
		}
	case 2:
		chip_pkg = "ESP32D2WD"
	case 4:
		chip_pkg = "ESP32-U4WDH"
	case 5:
		chip_pkg = "ESP32-PICO-D4"
	default:
		chip_pkg = fmt.Sprintf("ESP32?%d", cpkg)
	}

	return fmt.Sprintf("%s R%d", chip_pkg, chip_rev), nil
}
