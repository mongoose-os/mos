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
package esp8266

import (
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/flash/esp"
)

var (
	FlashSizeToId = map[string]int{
		// +1, to distinguish from null-value
		"4m":   1,
		"2m":   2,
		"8m":   3,
		"16m":  4,
		"32m":  5,
		"64m":  9,
		"128m": 10,
	}

	FlashSizes = map[int]int{
		0: 524288,
		1: 262144,
		2: 1048576,
		3: 2097152,
		4: 4194304,
		8: 8388608,
		9: 16777216,
	}
)

func GetChipDescr(rrw esp.RegReaderWriter) (string, error) {
	efuse0, err := rrw.ReadReg(0x3ff00050)
	if err != nil {
		return "", errors.Annotatef(err, "failed to read eFuse")
	}
	efuse2, err := rrw.ReadReg(0x3ff00058)
	if err != nil {
		return "", errors.Annotatef(err, "failed to read eFuse")
	}
	if efuse0&(1<<4) != 0 || efuse2&(1<<16) != 0 {
		return "ESP8285", nil
	}
	return "ESP8266EX", nil
}
