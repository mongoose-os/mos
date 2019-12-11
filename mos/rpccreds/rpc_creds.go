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
package rpccreds

import (
	"io/ioutil"
	"strings"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"
)

var (
	rpcCreds = flag.String("rpc-creds", "", `Either "username:passwd" or "@filename" which contains username:passwd`)
)

func GetRPCCreds() (username, passwd string, err error) {
	if len(*rpcCreds) > 0 && (*rpcCreds)[0] == '@' {
		filename := (*rpcCreds)[1:]
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return "", "", errors.Annotatef(err, "reading RPC creds file %s", filename)
		}

		return getRPCCredsFromString(strings.TrimSpace(string(data)))
	} else {
		return getRPCCredsFromString(*rpcCreds)
	}
}

func getRPCCredsFromString(s string) (username, passwd string, err error) {
	parts := strings.Split(s, ":")
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	} else {
		// TODO(dfrank): handle the case with nothing or only username provided,
		// and prompt the user for the missing parts.

		return "", "", errors.Errorf("Failed to get username and password: wrong RPC creds spec")
	}
}
