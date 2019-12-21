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
	"io/ioutil"
	"os"
	"strconv"

	"context"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/devutil"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/flash/esp"
	espFlasher "github.com/mongoose-os/mos/cli/flash/esp/flasher"
	flag "github.com/spf13/pflag"
)

func flashWrite(ctx context.Context, devConn dev.DevConn) error {
	// if given devConn is not nil, we should disconnect it while flash writing is in progress
	if devConn != nil {
		devConn.Disconnect(ctx)
		defer devConn.Connect(ctx, true)
	}

	args := flag.Args()
	if len(args) != 3 {
		return errors.Errorf("address and file are required")
	}
	addr, err := strconv.ParseInt(args[1], 0, 64)
	if err != nil {
		return errors.Annotatef(err, "invalid address")
	}
	var data []byte
	inFile := args[2]
	if inFile == "-" {
		data, err = ioutil.ReadAll(os.Stdin)
	} else {
		data, err = ioutil.ReadFile(inFile)
	}
	if err != nil {
		return errors.Annotatef(err, "failed to read %s", inFile)
	}

	port, err := devutil.GetPort()
	if err != nil {
		return errors.Trace(err)
	}

	platform := flags.Platform()
	switch platform {
	case "esp32":
		espFlashOpts.ControlPort = port
		err = espFlasher.WriteFlash(esp.ChipESP32, uint32(addr), data, &espFlashOpts)
	case "esp8266":
		espFlashOpts.ControlPort = port
		err = espFlasher.WriteFlash(esp.ChipESP8266, uint32(addr), data, &espFlashOpts)
	default:
		err = errors.NotImplementedf("flash writing for %s", platform)
	}

	return errors.Trace(err)
}
