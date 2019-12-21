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
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/config"
	"github.com/mongoose-os/mos/cli/dev"
	flag "github.com/spf13/pflag"
)

func wifi(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()
	if len(args) != 3 {
		return errors.Errorf("Usage: %s wifi WIFI_NETWORK_NAME WIFI_PASSWORD", os.Args[0])
	}
	params := []string{
		"wifi.ap.enable=false",
		"wifi.sta.enable=true",
		fmt.Sprintf("wifi.sta.ssid=%s", args[1]),
		fmt.Sprintf("wifi.sta.pass=%s", args[2]),
	}
	return config.SetWithArgs(ctx, devConn, params)
}
