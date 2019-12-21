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
package devutil

import (
	"github.com/juju/errors"

	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
)

var defaultPort string

func GetPort() (string, error) {
	if *flags.Port != "auto" {
		return *flags.Port, nil
	}
	if defaultPort == "" {
		defaultPort = getDefaultPort()
		if defaultPort == "" {
			return "", errors.Errorf("--port not specified and none were found")
		}
		ourutil.Reportf("Using port %s", defaultPort)
	}
	return defaultPort, nil
}
