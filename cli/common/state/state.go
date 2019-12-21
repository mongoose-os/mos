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
package state

import (
	"encoding/json"
	"io/ioutil"

	"github.com/mongoose-os/mos/cli/common/paths"

	"github.com/juju/errors"
)

type State struct {
	Versions         map[string]*StateVersion `json:"versions"`
	OldDirsConverted bool                     `json:"old_dirs_converted"`
}

type StateVersion struct {
}

var (
	mosState State
)

func Init() error {
	// Try to read state from file, and if it succeeds, unmarshal json from it;
	// otherwise just leave state empty
	if data, err := ioutil.ReadFile(paths.StateFilepath); err == nil {
		if err := json.Unmarshal(data, &mosState); err != nil {
			return errors.Trace(err)
		}
	}

	if mosState.Versions == nil {
		mosState.Versions = make(map[string]*StateVersion)
	}

	return nil
}

func GetState() *State {
	return &mosState
}

func GetStateForVersion(version string) *StateVersion {
	return mosState.Versions[version]
}

func SetStateForVersion(version string, stateVer *StateVersion) {
	mosState.Versions[version] = stateVer
}

func SaveState() error {
	data, err := json.MarshalIndent(&mosState, "", "  ")
	if err != nil {
		return errors.Trace(err)
	}

	if err := ioutil.WriteFile(paths.StateFilepath, data, 0644); err != nil {
		return errors.Trace(err)
	}

	return nil
}
