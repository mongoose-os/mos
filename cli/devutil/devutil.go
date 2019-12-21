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
	"context"
	"crypto/tls"
	"runtime"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/common/mgrpc/codec"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/watson"
)

func createDevConnWithJunkHandler(ctx context.Context, junkHandler func(junk []byte)) (dev.DevConn, error) {
	port, err := GetPort()
	if err != nil {
		return nil, errors.Trace(err)
	}
	c := dev.Client{Port: port, Timeout: *flags.Timeout, Reconnect: *flags.Reconnect}
	prefix := "serial://"
	if strings.Index(port, "://") > 0 {
		prefix = ""
	}
	addr := prefix + port

	// Init and pass TLS config if --cert-file and --key-file are specified
	var tlsConfig *tls.Config
	if *flags.CertFile != "" ||
		strings.HasPrefix(port, "wss") ||
		strings.HasPrefix(port, "https") ||
		strings.HasPrefix(port, "mqtts") {

		tlsConfig, err = flags.TLSConfigFromFlags()
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	codecOpts := &codec.Options{
		AzureDM: codec.AzureDMCodecOptions{
			ConnectionString: *flags.AzureConnectionString,
		},
		GCP: codec.GCPCodecOptions{
			CreateTopic: *flags.GCPRPCCreateTopic,
		},
		MQTT: codec.MQTTCodecOptions{},
		Serial: codec.SerialCodecOptions{
			BaudRate:             uint(*flags.BaudRate),
			HardwareFlowControl:  *flags.HWFC,
			JunkHandler:          junkHandler,
			SetControlLines:      *flags.SetControlLines,
			InvertedControlLines: *flags.InvertedControlLines,
		},
		Watson: codec.WatsonCodecOptions{
			APIKey:       watson.WatsonAPIKeyFlag,
			APIAuthToken: watson.WatsonAPIAuthTokenFlag,
		},
	}
	// Due to lack of flow control, we send data in chunks and wait after each.
	// At non-default baud rate we assume user knows what they are doing.
	if !codecOpts.Serial.HardwareFlowControl &&
		codecOpts.Serial.BaudRate == 115200 &&
		!*flags.RPCUARTNoDelay {
		codecOpts.Serial.SendChunkSize = 16
		codecOpts.Serial.SendChunkDelay = 5 * time.Millisecond
		// So, this is weird. ST-Link serial device seems to have issues on Mac OS X if we write too fast.
		// Yes, even 16 bytes at 5 ms interval is too fast, and 8 bytes too. It looks like this:
		// processes trying to access /dev/cu.usbmodemX get stuck and the only re-plugging
		// (or re-enumerating) gets it out of this state.
		// Hence, the following kludge.
		if runtime.GOOS == "darwin" && strings.Contains(port, "cu.usbmodem") {
			codecOpts.Serial.SendChunkSize = 6
		}
	}
	devConn, err := c.CreateDevConnWithOpts(ctx, addr, *flags.Reconnect, tlsConfig, codecOpts)
	return devConn, errors.Trace(err)
}

func CreateDevConnFromFlags(ctx context.Context) (dev.DevConn, error) {
	return createDevConnWithJunkHandler(ctx, func(junk []byte) {})
}
