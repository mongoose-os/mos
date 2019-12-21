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
package license

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/common/paths"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
	flag "github.com/spf13/pflag"
)

type licenseServerAccess struct {
	Server string `json:"server,omitempty"`
	Key    string `json:"key,omitempty"`
}

type authFile struct {
	LicenseServerAccess []*licenseServerAccess `json:"license_server_access,omitempty"`
}

func readKey(server string) string {
	var auth authFile
	data, err := ioutil.ReadFile(paths.AuthFilepath)
	if err == nil {
		json.Unmarshal(data, &auth)
	}
	for _, s := range auth.LicenseServerAccess {
		if s.Server == server {
			return s.Key
		}
	}
	return ""
}

func promptKey(server string) {
	fmt.Printf(`
License server key not found.

1. Log in to %s
2. Click 'Key' in the top menu and copy the access key
3. Run "mos license-save-key ACCESS_KEY"
4. Re-run "mos license"
`+"\n", server)
}

func saveKey(server, key string) error {
	var auth authFile
	data, err := ioutil.ReadFile(paths.AuthFilepath)
	if err == nil {
		json.Unmarshal(data, &auth)
	}
	updated := false
	for _, s := range auth.LicenseServerAccess {
		if s.Server == server {
			s.Key = key
			updated = true
		}
	}
	if !updated {
		auth.LicenseServerAccess = append(auth.LicenseServerAccess, &licenseServerAccess{
			Server: server,
			Key:    key,
		})
	}
	data, _ = json.MarshalIndent(auth, "", "  ")
	if err = ioutil.WriteFile(paths.AuthFilepath, data, 0600); err == nil {
		ourutil.Reportf("Saved key for %s to %s", server, paths.AuthFilepath)
	}
	return err
}

func SaveKey(ctx context.Context, devConn dev.DevConn) error {
	key := *flags.LicenseServerKey
	if key == "" && len(flag.Args()) == 2 {
		key = flag.Args()[1]
	} else {
		return errors.Errorf("key is required %d", len(flag.Args()))
	}
	return saveKey(*flags.LicenseServer, key)
}
