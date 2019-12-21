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
		return "", errors.Trace(err)
	}
	cver, err := fusesByName["chip_package"].Value(false)
	if err != nil {
		return "", errors.Trace(err)
	}
	chip_ver := ""
	switch cver.Uint64() {
	case 0:
		chip_ver = "ESP32D0WDQ6"
	case 1:
		chip_ver = "ESP32D0WDQ5"
	case 2:
		chip_ver = "ESP32D2WDQ5"
	case 4:
		chip_ver = "ESP32-PICO-D2"
	case 5:
		chip_ver = "ESP32-PICO-D4"
	default:
		chip_ver = fmt.Sprintf("ESP32?%d", cver.Uint64())
	}

	crev, err := fusesByName["chip_ver_rev1"].Value(false)
	if err != nil {
		return "", errors.Trace(err)
	}

	return fmt.Sprintf("%s R%d", chip_ver, crev), nil
}
