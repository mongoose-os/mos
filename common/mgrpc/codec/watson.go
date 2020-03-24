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
package codec

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

// IBM Watson IoT Platform RPC support.
// It's an MQTT codec with a few twists.
// Basically, outgoing requests are sent as commands and responses are received as events.
// Command id is mgrpc-DEVICE_ID and event id is the same.

// URIs should be of the following format:
//   watson://org-or-host/devtype/devid
// org-or-host is either org id or full messaging host name, i.e. it can be
// myorg or myorg.messaging.internetofthings.ibmcloud.com
// API key and auth token can be provided as user:password component of the URI:
//   watson://a-myorg-4svdriwzyr:AAA%28-BBBCCDEFFRWD@myorg/mos/esp8266_123456
// Note that special symbols in the token, if present, must be URL-encoded.

const (
	WatsonURLScheme = "watson"
)

type WatsonCodecOptions struct {
	AppID        string // a random one will be generated if not set
	APIKey       string
	APIAuthToken string
}

func NewWatsonCodec(dst string, tlsConfig *tls.Config, co *WatsonCodecOptions) (Codec, error) {
	u, err := url.Parse(dst)
	if err != nil {
		return nil, errors.Trace(err)
	}
	pp := strings.Split(u.Path, "/")
	if len(pp) != 3 {
		return nil, errors.Errorf("invalid URI format: path must be devtype/devid")
	}

	orgId, host := "", ""
	if strings.Contains(u.Host, ".") {
		orgId = strings.SplitN(u.Host, ".", 2)[0]
		host = u.Host
	} else {
		orgId = u.Host
		host = fmt.Sprintf("%s.messaging.internetofthings.ibmcloud.com", orgId)
	}

	appId := co.AppID
	if appId == "" {
		appId = fmt.Sprintf("mos-%d", rand.Int31())
	}
	devType, devId := pp[1], pp[2]

	apiKey, apiAuthToken := co.APIKey, co.APIAuthToken
	if u.User != nil {
		apiKey = u.User.Username()
		passwd, isset := u.User.Password()
		if isset {
			apiAuthToken = passwd
		}
	}

	if apiKey == "" || apiAuthToken == "" {
		return nil, errors.Errorf("API key or token not provided")
	}

	murl := fmt.Sprintf("mqtts://%s/%s", host, devId)
	mopts := &MQTTCodecOptions{
		Src:      appId,
		User:     apiKey,
		ClientID: fmt.Sprintf("a:%s:%s", orgId, appId),
		PubTopic: fmt.Sprintf("iot-2/type/%s/id/%s/cmd/mgrpc-%s/fmt/json", devType, devId, devId),
		SubTopic: fmt.Sprintf("iot-2/type/%s/id/+/evt/mgrpc-%s/fmt/json", devType, appId),
	}
	glog.V(1).Infof("URL: %s, opts: %+v", murl, *mopts)
	mopts.Password = apiAuthToken

	return MQTT(murl, tlsConfig, mopts)
}
