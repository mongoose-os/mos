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
	"github.com/juju/errors"

	"github.com/mongoose-os/mos/cli/flash/esp"
)

func WriteFlash(ct esp.ChipType, addr uint32, data []byte, opts *esp.FlashOpts) error {
	cfr, err := ConnectToFlasherClient(ct, opts)
	if err != nil {
		return errors.Trace(err)
	}
	defer cfr.rc.Disconnect()
	im := &image{
		Name:         "image",
		Type:         "user",
		Addr:         addr,
		Data:         data,
		ESP32Encrypt: (opts.ESP32EncryptionKeyFile != ""),
	}
	return errors.Trace(writeImages(ct, cfr, []*image{im}, opts, false))
}
