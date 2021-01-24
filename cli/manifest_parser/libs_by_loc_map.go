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

import (
	"sync"

	"github.com/mongoose-os/mos/cli/build"
)

type libByLoc struct {
	Lib *build.SWModule
	mtx sync.Mutex
}

type libByLocMap struct {
	m   map[string]*libByLoc
	mtx sync.Mutex
}

func newLibByLocMap() *libByLocMap {
	return &libByLocMap{
		m:   map[string]*libByLoc{},
	}
}

// AddOrFetchAndLock() tries to add a new location key to the set.  If
// successful, the new entry (Lib: nil) is locked and returned; otherwise (the
// location key already exists) the pre-existing entry is locked and returned.
func (lm *libByLocMap) AddOrFetchAndLock(loc string) *libByLoc {
	lm.mtx.Lock()
	defer lm.mtx.Unlock()

	ls, ok := lm.m[loc]
	if !ok {
	  ls = &libByLoc{}
	  lm.m[loc] = ls
	}

	ls.mtx.Lock()
	return ls
}
