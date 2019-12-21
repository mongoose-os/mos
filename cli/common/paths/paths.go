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
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"

	"github.com/mongoose-os/mos/version"
)

var (
	dirTplMosVersion = "${mos.version}"

	AppsDirTpl = fmt.Sprintf("~/.mos/apps-%s", dirTplMosVersion)

	TmpDir         = ""
	depsDirFlag    = ""
	LibsDirFlag    = []string{}
	AppsDir        = ""
	modulesDirFlag = ""

	StateFilepath = ""
	AuthFilepath  = ""
)

func init() {
	flag.StringVar(&TmpDir, "temp-dir", "~/.mos/tmp", "Directory to store temporary files")
	flag.StringVar(&depsDirFlag, "deps-dir", "", "Directory to fetch libs, modules into")
	flag.StringSliceVar(&LibsDirFlag, "libs-dir", []string{}, "Directory to find libs in. Can be used multiple times.")
	flag.StringVar(&AppsDir, "apps-dir", AppsDirTpl, "Directory to store apps into")
	flag.StringVar(&modulesDirFlag, "modules-dir", "", "Directory to store modules into")

	flag.StringVar(&StateFilepath, "state-file", "~/.mos/state.json", "Where to store internal mos state")
	flag.StringVar(&AuthFilepath, "auth-file", "~/.mos/auth.json", "Where to store license server auth key")
}

// Init() should be called after all flags are parsed
func Init() error {
	var err error
	TmpDir, err = NormalizePath(TmpDir, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	depsDirFlag, err = NormalizePath(depsDirFlag, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	for i, s := range LibsDirFlag {
		LibsDirFlag[i], err = NormalizePath(s, version.GetMosVersion())
		if err != nil {
			return errors.Trace(err)
		}
	}

	AppsDir, err = NormalizePath(AppsDir, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	modulesDirFlag, err = NormalizePath(modulesDirFlag, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	StateFilepath, err = NormalizePath(StateFilepath, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	AuthFilepath, err = NormalizePath(AuthFilepath, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
	}

	if err := os.MkdirAll(TmpDir, 0777); err != nil {
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
	if depsDirFlag != "" {
		return depsDirFlag
	} else {
		return filepath.Join(projectDir, "deps")
	}
}

func GetModulesDir(projectDir string) string {
	if modulesDirFlag != "" {
		return modulesDirFlag
	} else {
		return filepath.Join(GetDepsDir(projectDir), "modules")
	}
}
