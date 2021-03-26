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
package build

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/juju/errors"
)

const DepsManifestVersion = "2021-03-26"

// DepsManifest describes exact versions of libraries, modules and binary blobs that went into firmware.
type DepsManifest struct {
	AppName string `yaml:"app_name,omitempty" json:"app_name,omitempty"`

	Libs    []*DepsManifestEntry `yaml:"libs,omitempty" json:"libs,omitempty"`
	Modules []*DepsManifestEntry `yaml:"modules,omitempty" json:"modules,omitempty"`

	ManifestVersion string `yaml:"manifest_version,omitempty" json:"manifest_version,omitempty"`
}

type DepsManifestEntry struct {
	Name        string           `yaml:"name,omitempty" json:"name,omitempty"`
	Location    string           `yaml:"location,omitempty" json:"location,omitempty"`
	Version     string           `yaml:"version,omitempty" json:"version,omitempty"`
	RepoVersion string           `yaml:"repo_version,omitempty" json:"repo_version,omitempty"`
	RepoDirty   bool             `yaml:"repo_dirty,omitempty" json:"repo_dirty,omitempty"`
	Blobs       []*DepsBlobEntry `yaml:"blobs,omitempty" json:"blobs,omitempty"`
}

type DepsBlobEntry struct {
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
	Size   int    `yaml:"size,omitempty" json:"size,omitempty"`
	SHA256 string `yaml:"cs_sha256,omitempty" json:"cs_sha256,omitempty"`
}

func GenerateDepsManifest(m *FWAppManifest) (*DepsManifest, error) {
	res := &DepsManifest{
		AppName:         m.Name,
		ManifestVersion: DepsManifestVersion,
	}

	for _, lh := range m.LibsHandled {
		e := &DepsManifestEntry{
			Name:        lh.Lib.Name,
			Location:    lh.Lib.Location,
			Version:     lh.Version,
			RepoVersion: lh.RepoVersion,
			RepoDirty:   lh.RepoDirty,
		}
		for _, fname := range lh.BinaryLibs {
			data, err := ioutil.ReadFile(fname)
			if err != nil {
				return nil, errors.Annotatef(err, "failed to checksum binary lib file")
			}
			dataHash := sha256.Sum256(data)
			e.Blobs = append(e.Blobs, &DepsBlobEntry{
				Name:   filepath.Base(fname),
				Size:   len(data),
				SHA256: fmt.Sprintf("%x", dataHash),
			})
		}
		sort.Slice(e.Blobs, func(i, j int) bool {
			return strings.Compare(e.Blobs[i].Name, e.Blobs[j].Name) < 0
		})
		res.Libs = append(res.Libs, e)
	}
	sort.Slice(res.Libs, func(i, j int) bool {
		return strings.Compare(res.Libs[i].Name, res.Libs[j].Name) < 0
	})

	for _, m := range m.Modules {
		rv, dirty, _ := m.GetRepoVersion()
		res.Modules = append(res.Modules, &DepsManifestEntry{
			Name:        m.Name,
			Location:    m.Location,
			RepoVersion: rv,
			RepoDirty:   dirty,
		})
	}
	sort.Slice(res.Modules, func(i, j int) bool {
		return strings.Compare(res.Modules[i].Name, res.Modules[j].Name) < 0
	})

	return res, nil
}
