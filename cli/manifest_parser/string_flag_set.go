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
package manifest_parser

import "sync"

type stringFlagSet struct {
	m   map[string]struct{}
	mtx sync.Mutex
}

func newStringFlagSet() *stringFlagSet {
	return &stringFlagSet{
		m:   map[string]struct{}{},
		mtx: sync.Mutex{},
	}
}

// Add tries to add a new key to the set. If key was added, returns true;
// otherwise (key already exists) returns false.
func (fs *stringFlagSet) Add(key string) bool {
	fs.mtx.Lock()
	defer fs.mtx.Unlock()

	_, ok := fs.m[key]
	if !ok {
		fs.m[key] = struct{}{}
		return true
	}

	return false
}
