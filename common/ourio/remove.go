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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cesanta/errors"
)

// RemoveFromDir removes everything from the given dir, except items with
// blacklisted names
func RemoveFromDir(dir string, blacklist []string) (err error) {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		err = errors.Trace(err)
		return
	}

entriesLoop:
	for _, entry := range entries {
		for _, v := range blacklist {
			if entry.Name() == v {
				// Current entry is blacklisted, skip
				continue entriesLoop
			}
		}

		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}
