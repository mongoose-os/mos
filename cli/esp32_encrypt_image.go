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
// +build !noflash

package main

import (
	"context"
	"io/ioutil"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flash/esp32"
	flag "github.com/spf13/pflag"
)

var (
	esp32FlashAddress uint32
)

func init() {
	flag.Uint32Var(&esp32FlashAddress, "esp32-flash-address", 0, "")
}

func esp32EncryptImage(ctx context.Context, devConn dev.DevConn) error {
	if len(flag.Args()) != 3 {
		return errors.Errorf("input and output images are required")
	}
	inFile := flag.Args()[1]
	outFile := flag.Args()[2]
	inData, err := ioutil.ReadFile(inFile)
	if err != nil {
		return errors.Annotatef(err, "failed to read input file")
	}
	key, err := ioutil.ReadFile(espFlashOpts.ESP32EncryptionKeyFile)
	if err != nil {
		return errors.Annotatef(err, "failed to read encryption key")
	}
	outData, err := esp32.ESP32EncryptImageData(
		inData, key, esp32FlashAddress, espFlashOpts.ESP32FlashCryptConf)
	if err != nil {
		return errors.Annotatef(err, "failed to encrypt data")
	}
	err = ioutil.WriteFile(outFile, outData, 0644)
	if err != nil {
		return errors.Annotatef(err, "failed to write output")
	}
	return nil
}
