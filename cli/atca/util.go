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
package atca

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"context"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/ourutil"
)

const (
	KeyFilePrefix = "ATCA:"
)

func Connect(ctx context.Context, dc dev.DevConn) ([]byte, *Config, error) {
	var r GetConfigResult

	if err := dc.Call(ctx, "ATCA.GetConfig", nil, &r); err != nil {
		return nil, nil, errors.Annotatef(err, "GetConfig")
	}

	if r.Config == nil {
		return nil, nil, errors.New("no config data in response")
	}

	confData, err := base64.StdEncoding.DecodeString(*r.Config)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to decode config data")
	}
	if len(confData) != ConfigSize {
		return nil, nil, errors.Errorf("expected %d bytes, got %d", ConfigSize, len(confData))
	}

	cfg, err := ParseBinaryConfig(confData)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "ParseBinaryConfig")
	}

	var model string
	if cfg.Revision >= 0x6000 {
		model = "ATECC608A"
	} else {
		model = "ATECC508A"
	}
	ourutil.Reportf("\n%s rev 0x%x S/N 0x%s, config is %s, data is %s",
		model, cfg.Revision, hex.EncodeToString(cfg.SerialNum), strings.ToLower(string(cfg.LockConfig)),
		strings.ToLower(string(cfg.LockValue)))

	if cfg.LockConfig != LockModeLocked || cfg.LockValue != LockModeLocked {
		ourutil.Reportf("WARNING: Either config or data zone are not locked, chip is not fully configured")
	}
	ourutil.Reportf("")

	return confData, cfg, nil
}

func WriteHex(data []byte, numPerLine int) []byte {
	s := ""
	for i := 0; i < len(data); {
		for j := 0; j < numPerLine && i < len(data); j++ {
			comma := ""
			if i < len(data)-1 {
				comma = ", "
			}
			s += fmt.Sprintf("0x%02x%s", data[i], comma)
			i++
		}
		s += "\n"
	}
	return []byte(s)
}

func ReadHex(data []byte) []byte {
	var result []byte
	hexByteRegex := regexp.MustCompile(`[0-9a-fA-F]{2}`)
	for _, match := range hexByteRegex.FindAllString(string(data), -1) {
		b, _ := hex.DecodeString(match)
		result = append(result, b[0])
	}
	return result
}

func JSONStr(v interface{}) string {
	bb, _ := json.MarshalIndent(v, "", "  ")
	return string(bb)
}
