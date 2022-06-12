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

//go:generate go-bindata -pkg esp32c3 -nocompress -modtime 1 -mode 420 stub/stub.json

package esp32c3

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/flash/esp"
)

const (
	EFUSE_BASE = 0x60008800
)

func GetChipDescr(rr esp.RegReader) (string, error) {
	block1_word3, err := rr.ReadReg(EFUSE_BASE + 0x44 + (3 * 4))
	if err != nil {
		return "", errors.Annotatef(err, "failed to read eFuse reg")
	}
	chip_pkg := (block1_word3 >> 21) & 0x7
	var chip_pkg_str string
	switch chip_pkg {
	case 0:
		chip_pkg_str = "ESP32-C3"
	default:
		chip_pkg_str = fmt.Sprintf("?(%d", chip_pkg)
	}
	chip_rev := (block1_word3 >> 18) & 0x7
	return fmt.Sprintf("%s R%d", chip_pkg_str, chip_rev), nil
}
