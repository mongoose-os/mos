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
	UserVersion string           `yaml:"user_version,omitempty" json:"version,omitempty"`
	RepoVersion string           `yaml:"repo_version,omitempty" json:"repo_version,omitempty"`
	RepoDirty   bool             `yaml:"repo_dirty,omitempty" json:"repo_dirty,omitempty"`
	Blobs       []*DepsBlobEntry `yaml:"blobs,omitempty" json:"blobs,omitempty"`
}

type DepsBlobEntry struct {
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
	Size   int    `yaml:"size,omitempty" json:"size,omitempty"`
	SHA256 string `yaml:"cs_sha256,omitempty" json:"cs_sha256,omitempty"`
}

func findEntry(entries []*DepsManifestEntry, name string) *DepsManifestEntry {
	for _, e := range entries {
		if e.Name == name {
			return e
		}
	}
	return nil
}

func (dm *DepsManifest) FindLibEntry(name string) *DepsManifestEntry {
	return findEntry(dm.Libs, name)
}

func (dm *DepsManifest) FindBlobEntry(libName, blobName string) *DepsBlobEntry {
	l := dm.FindLibEntry(libName)
	if l == nil {
		return nil
	}
	for _, e := range l.Blobs {
		if e.Name == blobName {
			return e
		}
	}
	return nil
}

func (dm *DepsManifest) FindModuleEntry(name string) *DepsManifestEntry {
	return findEntry(dm.Modules, name)
}

func GenerateDepsManifest(manifest *FWAppManifest) (*DepsManifest, error) {
	res := &DepsManifest{
		AppName:         manifest.Name,
		ManifestVersion: DepsManifestVersion,
	}

	for _, lh := range manifest.LibsHandled {
		e := &DepsManifestEntry{
			Name:        lh.Lib.Name,
			Location:    lh.Lib.Location,
			Version:     lh.Version,
			UserVersion: lh.UserVersion,
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

	for _, m := range manifest.Modules {
		rv, dirty, _ := m.GetRepoVersion()
		res.Modules = append(res.Modules, &DepsManifestEntry{
			Name:        m.Name,
			Location:    m.Location,
			Version:     m.GetVersion(manifest.ModulesVersion),
			RepoVersion: rv,
			RepoDirty:   dirty,
		})
	}
	sort.Slice(res.Modules, func(i, j int) bool {
		return strings.Compare(res.Modules[i].Name, res.Modules[j].Name) < 0
	})

	return res, nil
}

func ValidateDepsRequirements(have, want *DepsManifest) error {
	// ReporVersion was enforced during fetch.
	var failures []string
	for _, haveLib := range have.Libs {
		name := haveLib.Name
		wantLib := want.FindLibEntry(name)
		if wantLib == nil {
			failures = append(failures, fmt.Sprintf("%s: no entry found", name))
			continue
		}
		if wantLib.UserVersion != "" && haveLib.UserVersion != wantLib.UserVersion {
			failures = append(failures, fmt.Sprintf("%s: want user version %s, have %s",
				name, wantLib.UserVersion, haveLib.UserVersion))
		}
		if wantLib.RepoVersion != "" && haveLib.RepoVersion != wantLib.RepoVersion {
			failures = append(failures, fmt.Sprintf("%s: want repo version %s, have %s",
				name, wantLib.RepoVersion, haveLib.RepoVersion))
		}
		if haveLib.RepoDirty && !wantLib.RepoDirty {
			failures = append(failures, fmt.Sprintf("%s: repo is dirty", name))
		}
		for _, haveBlob := range haveLib.Blobs {
			blobName := haveBlob.Name
			wantBlob := want.FindBlobEntry(name, blobName)
			if wantBlob == nil {
				failures = append(failures, fmt.Sprintf("%s: %s: no entry found", name, blobName))
				continue
			}
			// Can't enforce this for "latest" as binaries are rebuilt regularly.
			// TODO(rojer): Impelment archiving of "latest" blobs.
			if haveLib.Version != "latest" {
				if (haveBlob.Size > 0 && haveBlob.Size != wantBlob.Size) ||
					(haveBlob.SHA256 != "" && haveBlob.SHA256 != wantBlob.SHA256) {
					failures = append(failures, fmt.Sprintf("%s: %s: want %d/%s, have %d/%s",
						name, blobName, wantBlob.Size, wantBlob.SHA256, haveBlob.Size, haveBlob.SHA256))
				}
			}
		}
	}
	for _, haveMod := range have.Modules {
		name := haveMod.Name
		wantMod := want.FindModuleEntry(name)
		if wantMod == nil {
			failures = append(failures, fmt.Sprintf("%s: no entry found", name))
			continue
		}
		if wantMod.RepoVersion != "" && haveMod.RepoVersion != wantMod.RepoVersion {
			failures = append(failures, fmt.Sprintf("%s: want repo version %s, have %s",
				name, wantMod.RepoVersion, haveMod.RepoVersion))
		}
		if haveMod.RepoDirty && !wantMod.RepoDirty {
			failures = append(failures, fmt.Sprintf("%s: repo is dirty", name))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("deps validation failures: %s", strings.Join(failures, "; "))
	}
	return nil
}
