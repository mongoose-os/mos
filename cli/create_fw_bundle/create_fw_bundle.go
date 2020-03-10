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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"

	"github.com/mongoose-os/mos/common/fwbundle"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
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
			p, err := fwbundle.PartFromString(ps)
			if err != nil {
				return errors.Annotatef(err, "%s", ps)
			}
			if strings.HasSuffix(p.Src, ".hex") {
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
			} else {
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
	ourutil.Reportf("Writing %s", *flags.Output)
	return fwbundle.WriteZipFirmwareBundle(fwb, *flags.Output, *flags.Compress, extraAttrs)
}
