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
	"strconv"
	"strings"

	moscommon "github.com/mongoose-os/mos/mos/common"
	"github.com/mongoose-os/mos/mos/dev"
	"github.com/mongoose-os/mos/mos/flags"
	"github.com/mongoose-os/mos/mos/version"

	"github.com/cesanta/errors"
	"github.com/mongoose-os/mos/common/fwbundle"
	"github.com/mongoose-os/mos/mos/ourutil"
	flag "github.com/spf13/pflag"
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
		fwb.FirmwareManifest = *fm
	} else {
		fm.Name = *flags.Name
		fm.Platform = flags.Platform()
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
				hpp, err := fwbundle.PartsFromHexFile(p.Src, p.Name)
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
	attrs, err := moscommon.ParseParamValues(*flags.Attr)
	if err != nil {
		return errors.Annotatef(err, "failed to parse --attr")
	}
	for attr, valueStr := range attrs {
		var value interface{}
		switch valueStr {
		case "true":
			value = true
		case "false":
			value = false
		default:
			if i, err := strconv.ParseInt(valueStr, 0, 64); err == nil {
				value = i
			} else {
				value = valueStr
			}
		}
		fwb.SetAttr(attr, value)
	}
	ourutil.Reportf("Writing %s", *flags.Output)
	return fwbundle.WriteZipFirmwareBundle(fwb, *flags.Output, *flags.Compress)
}
