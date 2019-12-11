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
package ourio

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/juju/errors"
	yaml "gopkg.in/yaml.v2"
)

// WriteFileIfDiffers writes data to file but avoids overwriting a file with the same contents.
// Returns true if existing file was updated.
func WriteFileIfDifferent(filename string, data []byte, perm os.FileMode) (bool, error) {
	exData, err := ioutil.ReadFile(filename)

	if err == nil && bytes.Compare(exData, data) == 0 {
		return false, nil
	}

	if err2 := ioutil.WriteFile(filename, data, perm); err2 != nil {
		return false, errors.Trace(err2)
	}

	return (err == nil), nil
}

// WriteFileIfDiffers writes s as YAML to file but avoids overwriting a file with the same contents.
// Returns true if the file was updated.
func WriteYAMLFileIfDifferent(filename string, s interface{}, perm os.FileMode) (bool, error) {
	data, err := yaml.Marshal(s)
	if err != nil {
		return false, errors.Trace(err)
	}
	return WriteFileIfDifferent(filename, data, perm)
}
