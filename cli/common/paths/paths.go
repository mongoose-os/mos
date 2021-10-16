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
package paths

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"

	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/version"
)

var (
	dirTplMosVersion = "${mos.version}"

	AppsDirTpl = fmt.Sprintf("~/.mos/apps-%s", dirTplMosVersion)

	AppsDir = ""
)

func init() {
	flag.StringVar(&AppsDir, "apps-dir", AppsDirTpl, "Directory to store apps into")
}

// Init() should be called after all flags are parsed
func Init() error {
	var err error

	*flags.DepsDir, err = NormalizePath(*flags.DepsDir, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	for i, s := range *flags.LibsDir {
		(*flags.LibsDir)[i], err = NormalizePath(s, version.GetMosVersion())
		if err != nil {
			return errors.Trace(err)
		}
	}

	AppsDir, err = NormalizePath(AppsDir, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	*flags.ModulesDir, err = NormalizePath(*flags.ModulesDir, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	*flags.StateFile, err = NormalizePath(*flags.StateFile, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	*flags.AuthFile, err = NormalizePath(*flags.AuthFile, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func NormalizePath(p, version string) (string, error) {
	var err error

	if p == "" {
		return "", nil
	}

	// Replace tilda with the actual path to home directory
	if len(p) > 0 && p[0] == '~' {
		// Unfortunately user.Current() doesn't play nicely with static build, so
		// we have to get home directory from the environment
		homeEnvName := "HOME"
		if runtime.GOOS == "windows" {
			homeEnvName = "USERPROFILE"
		}
		p = os.Getenv(homeEnvName) + p[1:]
	}

	// Replace ${mos.version} with the actual version
	p = strings.Replace(p, dirTplMosVersion, version, -1)

	// Absolutize path
	p, err = filepath.Abs(p)
	if err != nil {
		return "", errors.Trace(err)
	}

	return p, nil
}

func GetDepsDir(projectDir string) string {
	if *flags.DepsDir != "" {
		return *flags.DepsDir
	} else {
		return filepath.Join(projectDir, "deps")
	}
}

func GetTempDir(subdir string) (string, error) {
	dir, err := NormalizePath(*flags.TempDir, version.GetMosVersion())
	if err != nil {
		return "", errors.Trace(err)
	}
	if err = os.MkdirAll(dir, 0777); err != nil {
		return "", errors.Trace(err)
	}
	if subdir != "" {
		dir, err = ioutil.TempDir(dir, subdir)
		if err != nil {
			return "", errors.Trace(err)
		}
	}
	return dir, nil
}

func GetModulesDir(projectDir string) string {
	if *flags.ModulesDir != "" {
		return *flags.ModulesDir
	} else {
		return filepath.Join(GetDepsDir(projectDir), "modules")
	}
}
