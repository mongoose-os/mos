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
package x509utils

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"

	"github.com/juju/errors"

	"github.com/mongoose-os/mos/cli/ourutil"
)

func WritePEM(derBytes []byte, blockType string, outputFileName string) error {
	var out io.Writer
	switch outputFileName {
	case "":
		out = os.Stdout
	case "-":
		out = os.Stdout
	case "--":
		out = os.Stderr
	default:
		f, err := os.OpenFile(outputFileName, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return errors.Annotatef(err, "failed to open %s for writing", outputFileName)
		}
		out = f
		defer func() {
			f.Close()
			ourutil.Reportf("Wrote %s", outputFileName)
		}()
	}
	pem.Encode(out, &pem.Block{Type: blockType, Bytes: derBytes})
	return nil
}

func WritePubKey(pubKey *ecdsa.PublicKey, outputFileName string) error {
	pubKeyDERBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return errors.Annotatef(err, "failed to marshal public key")
	}
	return WritePEM(pubKeyDERBytes, "PUBLIC KEY", outputFileName)
}
