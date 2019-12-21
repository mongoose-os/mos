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
package clone

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/build"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/version"
	flag "github.com/spf13/pflag"
)

func Clone(ctx context.Context, devConn dev.DevConn) error {
	var m build.SWModule

	args := flag.Args()
	switch len(args) {
	case 1:
		return errors.Errorf("source location is required")
	case 2:
		m.Location = args[1]
	case 3:
		m.Location = args[1]
		os.MkdirAll(filepath.Dir(args[2]), 0755)
		if err := os.Chdir(filepath.Dir(args[2])); err != nil {
			return errors.Trace(err)
		}
		m.Name = filepath.Base(args[2])
	default:
		return errors.Errorf("extra arguments")
	}

	switch m.GetType() {
	case build.SWModuleTypeInvalid:
		return errors.Errorf("invalid repo path")
	case build.SWModuleTypeLocal:
		// Makes no sense to clone local dir, so we assume it's a short GitHub repo name.
		if strings.Contains(m.Location, "/") {
			m.Location = fmt.Sprintf("https://github.com/%s", m.Location)
		} else {
			m.Location = fmt.Sprintf("https://github.com/mongoose-os-apps/%s", m.Location)
		}
	default:
		// Proceed as usual
	}

	d, err := m.PrepareLocalDir(".", os.Stderr, false, /* deleteIfFailed */
		version.GetMosVersion() /* defaultVersion */, 0 /* pullInterval */, 0 /* cloneDepth */)

	// Chdir is needed for the Web UI mode: immediately go into the cloned repo.
	if m.Name != "" {
		os.Rename(d, m.Name)
		os.Chdir(m.Name)
	} else {
		os.Chdir(d)
	}

	return err
}
