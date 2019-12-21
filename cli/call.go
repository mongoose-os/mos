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
	"encoding/json"
	"fmt"

	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"
)

func isJSONString(s string) bool {
	var js string
	return json.Unmarshal([]byte(s), &js) == nil
}

func isJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

func callDeviceService(
	ctx context.Context, devConn dev.DevConn, method string, args string,
) (string, error) {
	b, e := devConn.(*dev.MosDevConn).CallB(ctx, method, args)

	// TODO(dfrank): instead of that, we should probably add a separate function
	// for rebooting
	if method == "Sys.Reboot" {
		waitForReboot()
	}

	return string(b), e
}

func call(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()[1:]
	if len(args) < 1 {
		return errors.Errorf("method required")
	}

	params := ""
	if len(args) > 1 {
		params = args[1]
	}

	if *flags.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *flags.Timeout)
		defer cancel()
	}

	result, err := callDeviceService(ctx, devConn, args[0], params)
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}
