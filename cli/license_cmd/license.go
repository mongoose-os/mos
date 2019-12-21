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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/config"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/devutil"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/version"
)

type licenseRequest struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id"`
	App      string `json:"app,omitempty"`
}

type licenseResponse struct {
	Text    string `json:"text"`
	Left    int    `json:"left"`
	Message string `json:"message"`
}

func License(ctx context.Context, devConn dev.DevConn) error {
	var err error
	server := *flags.LicenseServer
	key := *flags.LicenseServerKey
	if key == "" {
		key = readKey(server)
	}
	if key == "" {
		promptKey(server)
		return errors.New("Failed to obtain access key")
	}
	lreq := licenseRequest{
		Type:     *flags.PID,
		DeviceID: *flags.UID,
		App:      "",
	}
	if lreq.DeviceID == "" {
		ourutil.Reportf("Connecting to the device...")
		devConn, err = devutil.CreateDevConnFromFlags(ctx)
		if err != nil {
			return errors.Annotatef(err, "error connecting to device")
		}
		ourutil.Reportf("Querying device UID...")
		var res struct {
			PID string `json:"pid"`
			UID string `json:"uid"`
			App string `json:"app"`
		}
		if err := devConn.Call(ctx, "Sys.GetUID", nil, &res); err != nil {
			return errors.Annotatef(err, "error querying device UID")
		}
		if res.UID == "" || res.PID == "" {
			return errors.Annotatef(err, "invalid device reply %s", res)
		}
		ourutil.Reportf("PID: %s UID: %s", res.PID, res.UID)
		lreq.Type = res.PID
		lreq.DeviceID = res.UID
		lreq.App = res.App
	}

	ourutil.Reportf("Requesting license from %s...", server)

	postData, _ := json.Marshal(&lreq)
	url := fmt.Sprintf("%s/api/v1/license", server)
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postData))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", key))
	req.Header.Add("User-Agent", version.GetUserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return errors.Annotatef(err, "HTTP request failed %s", url)
	}
	defer resp.Body.Close()
	rs, _ := ioutil.ReadAll(resp.Body)
	glog.V(1).Infof("Server response: %s", string(rs))
	var lresp licenseResponse
	json.Unmarshal(rs, &lresp)
	if lresp.Text == "" {
		return errors.Errorf("Error obtaining license: %s", lresp.Message)
	}
	ourutil.Reportf("License: %s (%d left)", lresp.Text, lresp.Left)
	if devConn == nil {
		return nil
	}
	devConf, err := dev.GetConfig(ctx, devConn)
	if err != nil {
		return errors.Annotatef(err, "failed to get config")
	}
	settings := map[string]string{
		"device.license": lresp.Text,
	}
	if err := config.ApplyDiff(devConf, settings); err != nil {
		return errors.Trace(err)
	}
	// Save to level 1 by default.
	level := *flags.Level
	if level < 0 {
		level = 1
	}
	return config.SetAndSaveLevel(ctx, devConn, devConf, level)
}
