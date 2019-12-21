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
package flasher

import (
	"time"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/flash/common"
	"github.com/mongoose-os/mos/cli/flash/esp"
)

func ReadFlash(ct esp.ChipType, addr uint32, length int, opts *esp.FlashOpts) ([]byte, error) {
	if addr < 0 {
		return nil, errors.Errorf("invalid addr: %d", addr)
	}
	if length < 0 {
		return nil, errors.Errorf("invalid addr: %d", length)
	}

	cfr, err := ConnectToFlasherClient(ct, opts)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer cfr.rc.Disconnect()

	flashSize := cfr.flashParams.Size()
	if addr == 0 && length == 0 {
		length = flashSize
	} else if int(addr)+length > flashSize {
		return nil, errors.Errorf("0x%x + %d exceeds flash size (%d)", addr, length, flashSize)
	}

	common.Reportf("Reading %d @ 0x%x...", length, addr)
	data := make([]byte, length)
	start := time.Now()
	if err := cfr.fc.Read(uint32(addr), data); err != nil {
		return nil, errors.Annotatef(err, "failed to read %d @ 0x%x", length, addr)
	}
	seconds := time.Since(start).Seconds()
	bytesPerSecond := float64(len(data)) / seconds
	common.Reportf("Read %d bytes in %.2f seconds (%.2f KBit/sec)", length, seconds, bytesPerSecond*8/1024)
	return data, nil
}
