package paths

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"cesanta.com/mos/version"

	"github.com/cesanta/errors"
)

var (
	dirTplMosVersion = "${mos.version}"

	AppsDirTpl = fmt.Sprintf("~/.mos/apps-%s", dirTplMosVersion)

	TmpDir         = ""
	depsDirFlag    = ""
	LibsDirFlag    = ""
	AppsDir        = ""
	modulesDirFlag = ""

	StateFilepath = ""
	AuthFilepath  = ""
)

func init() {
	flag.StringVar(&TmpDir, "temp-dir", "~/.mos/tmp", "Directory to store temporary files")
	flag.StringVar(&depsDirFlag, "deps-dir", "", "Directory to fetch libs, modules into")
	flag.StringVar(&LibsDirFlag, "libs-dir", "", "Directory to find libs in")
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

	LibsDirFlag, err = NormalizePath(LibsDirFlag, version.GetMosVersion())
	if err != nil {
		return errors.Trace(err)
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
