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
package fwbundle

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/ourutil"
)

const (
	FSPartType = "fs"
)

type FirmwareBundle struct {
	FirmwareManifest

	tempDir string
}

type firmwareManifest struct {
	Name           string                   `json:"name,omitempty"`
	Platform       string                   `json:"platform,omitempty"`
	Description    string                   `json:"description,omitempty"`
	Version        string                   `json:"version,omitempty"`
	BuildID        string                   `json:"build_id,omitempty"`
	BuildTimestamp *time.Time               `json:"build_timestamp,omitempty"`
	Parts          map[string]*FirmwarePart `json:"parts"`

	// Extra attributes.
	attrs map[string]interface{}
}

type FirmwareManifest firmwareManifest

func NewBundle() *FirmwareBundle {
	return &FirmwareBundle{}
}

func (fwb *FirmwareBundle) AddPart(p *FirmwarePart) error {
	if fwb.FirmwareManifest.Parts == nil {
		fwb.FirmwareManifest.Parts = make(map[string]*FirmwarePart)
	}
	fwb.FirmwareManifest.Parts[p.Name] = p
	return nil
}

type partsByAddr []*FirmwarePart

func (pp partsByAddr) Len() int      { return len(pp) }
func (pp partsByAddr) Swap(i, j int) { pp[i], pp[j] = pp[j], pp[i] }
func (pp partsByAddr) Less(i, j int) bool {
	return pp[i].Addr < pp[j].Addr
}

func (fwb *FirmwareBundle) PartsByAddr() []*FirmwarePart {
	var pp []*FirmwarePart
	for _, p := range fwb.Parts {
		if p.Flash == nil || *p.Flash {
			pp = append(pp, p)
		}
	}
	sort.Sort(partsByAddr(pp))
	return pp
}

func ReadManifest(fname string) (*FirmwareManifest, error) {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, errors.Annotatef(err, "ReadManifest(%s)", fname)
	}
	var fm FirmwareManifest
	if err := json.Unmarshal(data, &fm); err != nil {
		return nil, errors.Annotatef(err, "ReadManifest(%s)", fname)
	}
	return &fm, nil
}

func (fw *FirmwareBundle) GetTempDir() (string, error) {
	if fw.tempDir == "" {
		td, err := ioutil.TempDir("", fmt.Sprintf("%s_%s_%s_",
			ourutil.FileNameFromString(fw.Name),
			ourutil.FileNameFromString(fw.Platform),
			ourutil.FileNameFromString(fw.Version)))
		if err != nil {
			return "", errors.Annotatef(err, "failed to create temp dir")
		}
		fw.tempDir = td
	}

	return fw.tempDir, nil
}

func (fw *FirmwareBundle) GetPartData(name string) ([]byte, error) {
	p := fw.Parts[name]
	if p == nil {
		return nil, errors.Errorf("%q: no such part", name)
	}
	return p.GetData()
}

func (fw *FirmwareBundle) GetPartDataFile(name string) (string, int, error) {
	data, err := fw.GetPartData(name)
	if err != nil {
		return "", -1, errors.Trace(err)
	}

	td, err := fw.GetTempDir()
	if err != nil {
		return "", -1, errors.Trace(err)
	}

	fname := filepath.Join(td, ourutil.FileNameFromString(name))

	err = ioutil.WriteFile(fname, data, 0644)

	glog.V(3).Infof("Wrote %q to %q (%d bytes)", name, fname, len(data))

	if err != nil {
		return "", -1, errors.Annotatef(err, "failed to write fw part data")
	}

	return fname, len(data), nil
}

func (fw *FirmwareBundle) Cleanup() {
	if fw.tempDir != "" {
		glog.Infof("Cleaning up %q", fw.tempDir)
		os.RemoveAll(fw.tempDir)
	}
}

func (fwb *FirmwareBundle) SetAttr(attr string, value interface{}) {
	if fwb.FirmwareManifest.attrs == nil {
		fwb.FirmwareManifest.attrs = make(map[string]interface{})
	}
	fwb.FirmwareManifest.attrs[attr] = value
}

func (fwm *FirmwareManifest) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(firmwareManifest(*fwm))
	if err != nil {
		return nil, err
	}
	if len(fwm.attrs) == 0 {
		return b, nil
	}
	eb, err := json.Marshal(fwm.attrs)
	if err != nil {
		return nil, err
	}
	eb[0] = ','
	rb := append(b[:len(b)-1], eb...)
	return rb, nil
}

func (fwm *FirmwareManifest) UnmarshalJSON(b []byte) error {
	// Start by filling in the struct fields.
	var fwm1 firmwareManifest
	if err := json.Unmarshal(b, &fwm1); err != nil {
		return err
	}
	*fwm = FirmwareManifest(fwm1)
	// Re-parse as a generic map.
	var mp map[string]interface{}
	json.Unmarshal(b, &mp)
	// Find keys that are not fields in the struct and add them as attrs.
	for k, v := range mp {
		if !isJSONField(fwm, k) {
			if fwm.attrs == nil {
				fwm.attrs = make(map[string]interface{})
			}
			fwm.attrs[k] = v
		}
	}
	return nil
}
