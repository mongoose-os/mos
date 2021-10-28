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
package create_fw_bundle

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"
	"github.com/youmark/pkcs8"
	"golang.org/x/crypto/ssh/terminal"
	glog "k8s.io/klog/v2"

	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/common/fwbundle"
	"github.com/mongoose-os/mos/version"
)

func CreateFWBundle(ctx context.Context, devConn dev.DevConn) error {
	if *flags.Output == "" {
		return errors.Errorf("--output is required")
	}
	var err error
	var fwb *fwbundle.FirmwareBundle
	if *flags.Input != "" {
		ourutil.Reportf("Reading firmware bundle from %s", *flags.Input)
		fwb, err = fwbundle.ReadZipFirmwareBundle(*flags.Input)
		if err != nil {
			return errors.Annotatef(err, "failed to read input bundle")
		}
	} else {
		fwb = fwbundle.NewBundle()
	}
	fm := &fwb.FirmwareManifest
	if *flags.Manifest != "" {
		ourutil.Reportf("Reading manifest from %s", *flags.Manifest)
		fm, err = fwbundle.ReadManifest(*flags.Manifest)
		if err != nil {
			return errors.Annotatef(err, "error reading existing manifest")
		}
	}
	fwb.FirmwareManifest = *fm
	if *flags.Name != "" {
		fm.Name = *flags.Name
	}
	if flags.Platform() != "" {
		fm.Platform = flags.Platform()
	}
	if *flags.Description != "" {
		fm.Description = *flags.Description
	}
	if *flags.BuildInfo != "" {
		var bi version.VersionJson
		data, err := ioutil.ReadFile(*flags.BuildInfo)
		if err != nil {
			return errors.Annotatef(err, "error reading build info")
		}
		if err := json.Unmarshal(data, &bi); err != nil {
			return errors.Annotatef(err, "error parsing build info")
		}
		fm.Version = bi.BuildVersion
		fm.BuildID = bi.BuildId
		fm.BuildTimestamp = &bi.BuildTimestamp
	}
	if len(flag.Args()) > 1 {
		for _, ps := range flag.Args()[1:] {
			pn, p, err := fwbundle.PartFromString(ps)
			switch {
			case err != nil:
				return errors.Annotatef(err, "%s", ps)
			case p == nil:
				err = fwb.RemovePart(pn)
				glog.Infof("Removing partition %s: %v", pn, err)
			case strings.HasSuffix(p.Src, ".hex"):
				hpp, err := fwbundle.PartsFromHexFile(p.Src, p.Name, 255, 512)
				if err != nil {
					return errors.Annotatef(err, "%s", ps)
				}
				for ihp, hp := range hpp {
					p1 := *p
					if len(hpp) == 1 {
						p1.Src = strings.Replace(p.Src, ".hex", ".bin", 1)
					} else {
						p1.Src = fmt.Sprintf("%s.%d.bin", strings.Replace(p.Src, ".hex", "", 1), ihp)
					}
					p1.Addr = hp.Addr
					p1.Name = hp.Name
					p1.Size = hp.Size
					data, _ := hp.GetData()
					p1.SetData(data)
					fwb.AddPart(&p1)
				}
			default:
				p.SetDataProvider(func(name, src string) ([]byte, error) {
					srcAbs := src
					if !filepath.IsAbs(src) && *flags.SrcDir != "" {
						srcAbs = filepath.Join(*flags.SrcDir, src)
					}
					return ioutil.ReadFile(srcAbs)
				})
				fwb.AddPart(p)
			}
		}
	}
	attrs, err := moscommon.ParseParamValuesTyped(*flags.Attr)
	if err != nil {
		return errors.Annotatef(err, "failed to parse --attr")
	}
	for attr, value := range attrs {
		fwb.SetAttr(attr, value)
	}
	extraAttrs, err := moscommon.ParseParamValuesTyped(*flags.ExtraAttr)
	if err != nil {
		return errors.Annotatef(err, "failed to parse --extra-attr")
	}
	var signers []crypto.Signer
	for _, key := range *flags.SignKeys {
		var s crypto.Signer
		if key != "" {
			// TODO(rojer): ATCA support, maybe?
			privKeyBytes, err := getPEMBlock(key, "EC PRIVATE KEY")
			if err != nil {
				if errors.Cause(err) == x509.IncorrectPasswordError {
					return err
				}
				// Is it encrypted?
				encPrivKeyBytes, err := getPEMBlock(key, "ENCRYPTED PRIVATE KEY")
				if err != nil {
					return errors.Annotatef(err, "failed to read private key %q", key)
				}
				fmt.Printf("Password for %q: ", filepath.Base(key))
				passwd, err := terminal.ReadPassword(0 /* stdin */)
				if err != nil {
					return errors.Annotatef(err, "error reading password for key %q", key)
				}
				fmt.Printf("\n")
				ecPrivKey, err := pkcs8.ParsePKCS8PrivateKeyECDSA(encPrivKeyBytes, passwd)
				if err != nil {
					return errors.Annotatef(err, "error decrypting private key %q", key)
				}
				s = ecPrivKey
			} else {
				ecPrivKey, err := x509.ParseECPrivateKey(privKeyBytes)
				if err != nil {
					return errors.Annotatef(err, "failed to parse EC private key %q", key)
				}
				s = ecPrivKey
			}
		}
		signers = append(signers, s)
	}
	ourutil.Reportf("Writing %s", *flags.Output)
	return fwbundle.WriteSignedZipFirmwareBundle(fwb, *flags.Output, *flags.Compress, signers, extraAttrs)
}

func getPEMBlock(file string, blockType string) ([]byte, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	n := 0
	for {
		p, rest := pem.Decode(data)
		if p == nil {
			break
		}
		n++
		if p.Type == blockType {
			if x509.IsEncryptedPEMBlock(p) {
				fmt.Printf("Password for %q: ", filepath.Base(file))
				passwd, err := terminal.ReadPassword(0 /* stdin */)
				if err != nil {
					return nil, errors.Annotatef(err, "error reading password for key %q", file)
				}
				fmt.Printf("\n")
				if data, err := x509.DecryptPEMBlock(p, passwd); err != nil {
					return nil, errors.Annotatef(err, "error decrypting private key %q", file)
				} else {
					return data, nil
				}
			}
			return p.Bytes, nil
		}
		data = rest
	}
	if n == 0 {
		return nil, fmt.Errorf("%s is not a PEM file", file)
	}
	return nil, fmt.Errorf("no PEM type %s found in %s", blockType, file)
}
