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
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"
	glog "k8s.io/klog/v2"

	"github.com/mongoose-os/mos/cli/build"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/common/paths"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/interpreter"
	"github.com/mongoose-os/mos/cli/mosgit"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/cli/update"
	"github.com/mongoose-os/mos/common/fwbundle"
	"github.com/mongoose-os/mos/version"
)

// mos build specific advanced flags
var (
	// Don't want to move this to BuildParams to avoid trivial command line injection.
	buildCmdExtra = flag.StringArray("build-cmd-extra", []string{}, "extra make flags, added at the end of the make command. Can be used multiple times.")

	// In-memory buffer containing all the log messages.  It has to be
	// thread-safe, because it's used in compProviderReal, which is an
	// implementation of the manifest_parser.ComponentProvider interface, whose
	// methods are called concurrently.
	logBuf threadSafeBuffer

	// Log writer which always writes to the build.log file, os.Stderr and logBuf
	logWriterStderr io.Writer

	// The same as logWriterStderr, but skips os.Stderr unless --verbose is given
	logWriter io.Writer
)

const (
	projectDir = "."
)

func init() {
	hiddenFlags = append(hiddenFlags, "docker_images")
}

// Build {{{

func getCredentialsFromCLI() (map[string]build.Credentials, error) {
	credsStr := *flags.Credentials
	if credsStr == "" && *flags.GHToken != "" {
		credsStr = *flags.GHToken
		glog.Errorf("--gh-token is deprecated, please use --credentials")
	}
	if credsStr == "" {
		return nil, nil
	}
	var entries []string
	if strings.HasPrefix(credsStr, "@") {
		f, err := os.Open(credsStr[1:])
		if err != nil {
			return nil, errors.Annotatef(err, "failed to open token file")
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			entries = append(entries, scanner.Text())
		}
	} else {
		entries = strings.Split(credsStr, ",")
	}
	result := map[string]build.Credentials{}
	for _, e := range entries {
		host := ""
		creds := build.Credentials{User: "mos"}
		parts := strings.Split(e, ":")
		switch len(parts) {
		case 1:
			// No host specified, used unless a more specific entry exists.
			host = ""
			creds.Pass = strings.TrimSpace(parts[0])
		case 2:
			// host:token
			host = strings.TrimSpace(parts[0])
			creds.Pass = strings.TrimSpace(parts[1])
		case 3:
			// host:user:password
			host = strings.TrimSpace(parts[0])
			creds.User = strings.TrimSpace(parts[1])
			creds.Pass = strings.TrimSpace(parts[2])
		default:
			return nil, errors.Errorf("invalid credentials entry %q", e)
		}
		result[host] = creds
	}
	return result, nil
}

// Build command handler {{{
func buildHandler(ctx context.Context, devConn dev.DevConn) error {
	var bParams build.BuildParams
	if *flags.BuildParams != "" {
		buildParamsBytes, err := ioutil.ReadFile(*flags.BuildParams)
		if err != nil {
			return errors.Annotatef(err, "error reading --build-params file")
		}
		if err := yaml.Unmarshal(buildParamsBytes, &bParams); err != nil {
			return errors.Annotatef(err, "error parsing --build-params file")
		}
	} else {
		// Create map of given lib locations, via --lib flag(s)
		cll, err := getCustomLocations(*flags.Libs)
		if err != nil {
			return errors.Trace(err)
		}

		// Create map of given module locations, via --module flag(s)
		cml, err := getCustomLocations(*flags.Modules)
		if err != nil {
			return errors.Trace(err)
		}
		if *flags.MosRepo != "" {
			cml[build.MosModuleName] = *flags.MosRepo
		}

		buildVarsFromCLI, err := getBuildVarsFromCLI()
		if err != nil {
			return errors.Trace(err)
		}

		cdefsFromCLI, err := getCdefsFromCLI()
		if err != nil {
			return errors.Trace(err)
		}

		libsFromCLI, err := getLibsFromCLI()
		if err != nil {
			return errors.Trace(err)
		}

		credentials, err := getCredentialsFromCLI()
		if err != nil {
			return errors.Annotatef(err, "error parsing --credentials")
		}

		libsUpdateIntvl := *flags.LibsUpdateInterval
		if *flags.NoLibsUpdate {
			libsUpdateIntvl = 0
		}

		bParams = build.BuildParams{
			ManifestAdjustments: build.ManifestAdjustments{
				Platform:  flags.Platform(),
				BuildVars: buildVarsFromCLI,
				CDefs:     cdefsFromCLI,
				CFlags:    *flags.CFlagsExtra,
				CXXFlags:  *flags.CXXFlagsExtra,
				ExtraLibs: libsFromCLI,
			},
			Clean:                 *flags.Clean,
			DryRun:                *flags.BuildDryRun,
			Verbose:               *flags.Verbose,
			BuildTarget:           *flags.BuildTarget,
			CustomLibLocations:    cll,
			CustomModuleLocations: cml,
			LibsUpdateInterval:    libsUpdateIntvl,
			NoPlatformCheck:       *flags.NoPlatformCheck,
			SaveBuildStat:         *flags.SaveBuildStat,
			PreferPrebuiltLibs:    *flags.PreferPrebuiltLibs,
			Credentials:           credentials,
		}
	}

	return errors.Trace(doBuild(ctx, &bParams))
}

func doBuild(ctx context.Context, bParams *build.BuildParams) error {
	var err error
	buildDir := moscommon.GetBuildDir(projectDir)

	if bParams.BuildTarget == "" {
		bParams.BuildTarget = moscommon.BuildTargetDefault
	}

	start := time.Now()

	// Request server version in parallel
	serverVersionCh := make(chan *version.VersionJson, 1)
	if true || !*flags.Local {
		go func() {
			v, err := update.GetServerMosVersion(string(update.GetUpdateChannel()), bParams.Platform, bParams.BuildVars["BOARD"])
			if err != nil {
				// Ignore error, it's not really important
				return
			}
			serverVersionCh <- v
		}()
	}

	if err := os.MkdirAll(buildDir, 0777); err != nil {
		return errors.Trace(err)
	}

	blog := moscommon.GetBuildLogFilePath(buildDir)
	logFile, err := os.Create(blog)
	if err != nil {
		return errors.Trace(err)
	}

	// Remove local log, ignore any errors
	os.RemoveAll(moscommon.GetBuildLogLocalFilePath(buildDir))

	logWriterStderr = io.MultiWriter(logFile, &logBuf, os.Stderr)
	logWriter = io.MultiWriter(logFile, &logBuf)

	if bParams.Verbose {
		logWriter = logWriterStderr
	}

	// Fail fast if there is no manifest
	if _, err := os.Stat(moscommon.GetManifestFilePath(projectDir)); os.IsNotExist(err) {
		return errors.Errorf("No mos.yml file")
	}

	if *flags.Local {
		err = buildLocal(ctx, bParams)
	} else {
		err = buildRemote(bParams)
	}
	if err != nil {
		return errors.Trace(err)
	}
	if bParams.DryRun {
		return nil
	}

	if bParams.BuildTarget == moscommon.BuildTargetDefault {
		// We were building a firmware, so perform the required actions with moving
		// firmware around, etc.
		fwFilename := moscommon.GetFirmwareZipFilePath(buildDir)

		fw, err := fwbundle.ReadZipFirmwareBundle(fwFilename)
		if err != nil {
			return errors.Trace(err)
		}

		end := time.Now()

		if bParams.SaveBuildStat {
			bstat := moscommon.BuildStat{
				ArchOld:     fw.Platform,
				Platform:    fw.Platform,
				AppName:     fw.Name,
				BuildTimeMS: int(end.Sub(start) / time.Millisecond),
			}

			data, err := json.MarshalIndent(&bstat, "", "  ")
			if err != nil {
				return errors.Trace(err)
			}

			ioutil.WriteFile(moscommon.GetBuildStatFilePath(buildDir), data, 0666)
		}

		if *flags.Local || !bParams.Verbose {
			if err == nil {
				freportf(logWriter, "Success, built %s/%s version %s (%s).", fw.Name, fw.Platform, fw.Version, fw.BuildID)
			}

			fullPath, _ := filepath.Abs(fwFilename)
			freportf(logWriterStderr, "Firmware saved to %s", fullPath)
		}
	} else if p := moscommon.GetOrigLibArchiveFilePath(buildDir, bParams.Platform); bParams.BuildTarget == p {
		freportf(logWriterStderr, "Lib saved to %s", moscommon.GetLibArchiveFilePath(buildDir))
	} else {
		// We were building some custom target, so just report that we succeeded.
		freportf(logWriterStderr, "Target %s is built successfully", bParams.BuildTarget)
	}

	// If received server version, compare it with the local one and notify the
	// user about the update (if available)
	select {
	case v := <-serverVersionCh:
		serverVer := v.BuildVersion
		localVer := version.Version

		if (update.GetUpdateChannel() == update.UpdateChannelRelease && serverVer != localVer) ||
			(update.GetUpdateChannel() == update.UpdateChannelLatest && strings.Compare(serverVer, localVer) > 0) {
			freportf(logWriterStderr, "By the way, there is a newer version available: %q (you use %q). "+
				`Run "mos update" to upgrade.`, serverVer, localVer)
		}
	default:
	}

	return err
}

func parseVarsSlice(varsSlice []string, vars map[string]string) error {
	for _, v := range varsSlice {
		pp1 := strings.SplitN(v, ":", 2)
		pp2 := strings.SplitN(v, "=", 2)
		var pp []string
		switch {
		case len(pp1) == 2 && len(pp2) == 1:
			pp = pp1
		case len(pp1) == 1 && len(pp2) == 2:
			pp = pp2
		case len(pp1) == 2 && len(pp2) == 2:
			if len(pp1[0]) < len(pp2[0]) {
				pp = pp1
			} else {
				pp = pp2
			}
		default:
			return errors.Errorf("invalid var specification: %q", v)
		}
		vars[pp[0]] = pp[1]
	}
	return nil
}

func getBuildVarsFromCLI() (map[string]string, error) {
	m := map[string]string{
		"BOARD": *flags.Board,
	}
	if err := parseVarsSlice(*flags.BuildVars, m); err != nil {
		return nil, errors.Annotatef(err, "invalid --build-var")
	}
	return m, nil
}

func getCdefsFromCLI() (map[string]string, error) {
	m := map[string]string{}
	if err := parseVarsSlice(*flags.CDefs, m); err != nil {
		return nil, errors.Annotatef(err, "invalid --cdef")
	}
	return m, nil
}

func getLibsFromCLI() ([]build.SWModule, error) {
	var res []build.SWModule
	for _, v := range *flags.LibsExtra {
		var m build.SWModule
		if err := yaml.Unmarshal([]byte(v), &m); err != nil {
			return nil, errors.Annotatef(err, "invalid --libs-extra value %q", v)
		}
		res = append(res, m)
	}
	return res, nil
}

type fileTransformer func(r io.ReadCloser) (io.ReadCloser, error)

func fixupAppName(appName string) (string, error) {
	if appName == "" {
		wd, err := getCodeDirAbs()
		if err != nil {
			return "", errors.Trace(err)
		}
		appName = filepath.Base(wd)
	}

	for _, c := range appName {
		if unicode.IsSpace(c) {
			return "", errors.Errorf("app name (%q) should not contain spaces", appName)
		}
	}

	return appName, nil
}

func getCodeDirAbs() (string, error) {
	absCodeDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", errors.Trace(err)
	}

	absCodeDir, err = filepath.EvalSymlinks(absCodeDir)
	if err != nil {
		return "", errors.Trace(err)
	}

	for _, c := range absCodeDir {
		if unicode.IsSpace(c) {
			return "", errors.Errorf("code dir (%q) should not contain spaces", absCodeDir)
		}
	}

	return absCodeDir, nil
}

func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme != ""
}

func getCustomLocations(entries []string) (map[string]string, error) {
	var err error
	res := map[string]string{}
	for _, l := range entries {
		parts := strings.SplitN(l, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --libs entry %q", l)
		}
		loc := parts[1]
		// Absolutize local paths
		if !isURL(loc) {
			loc, err = filepath.Abs(loc)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		res[parts[0]] = loc
	}
	return res, nil
}

func newMosVars() *interpreter.MosVars {
	ret := interpreter.NewMosVars()
	ret.SetVar(interpreter.GetMVarNameMosVersion(), version.GetMosVersion())
	return ret
}

func absPathSlice(slice []string) ([]string, error) {
	ret := make([]string, len(slice))
	for i, v := range slice {
		var err error
		if !filepath.IsAbs(v) {
			ret[i], err = filepath.Abs(v)
			if err != nil {
				return nil, errors.Trace(err)
			}
		} else {
			ret[i] = v
		}
	}
	return ret, nil
}

// manifest_parser.ComponentProvider implementation {{{
type compProviderReal struct {
	bParams   *build.BuildParams
	logWriter io.Writer
}

func (lpr *compProviderReal) GetLibLocalPath(
	m *build.SWModule, rootAppDir, libsDefVersion, platform string,
) (string, error) {

	name, err := m.GetName()
	if err != nil {
		return "", errors.Trace(err)
	}

	creds := lpr.bParams.GetCredentialsForHost(m.GetHostName())
	m.SetCredentials(creds)

	gitinst := mosgit.NewOurGit(build.BuildCredsToGitCreds(creds))

	appDir, err := getCodeDirAbs()
	if err != nil {
		return "", errors.Trace(err)
	}

	// --lib has the highest precedence.
	customLoc, ok := lpr.bParams.CustomLibLocations[name]
	if ok && !isURL(customLoc) {
		ourutil.Freportf(lpr.logWriter, "%s: Using %q (--lib)", name, customLoc)
		return customLoc, nil
	}

	// Check --libs-dir.
	if !ok && len(paths.LibsDirFlag) > 0 {
		name2, _ := m.GetName2()
		for _, libsDir := range paths.LibsDirFlag {
			libDir := filepath.Join(libsDir, name2)
			glog.V(2).Infof("%s (%s): Trying %s...", name, name2, libDir)
			if fi, err := os.Stat(libDir); err == nil && fi.IsDir() {
				ourutil.Freportf(lpr.logWriter, "%s: Using %q (--libs-dir)", name, libDir)
				return libDir, nil
			}
		}
	}

	// Try to fetch into --deps-dir.
	if ok {
		m.Location = customLoc
	}
	libDirAbs := ""
	depsDir := paths.GetDepsDir(appDir)
	for {
		localDir, err := m.GetLocalDir(depsDir, libsDefVersion)
		if err != nil {
			return "", errors.Trace(err)
		}

		updateIntvl := lpr.bParams.LibsUpdateInterval

		// Try to get current hash, ignoring errors
		curHash := ""
		if m.GetType() == build.SWModuleTypeGit {
			curHash, _ = gitinst.GetCurrentHash(localDir)
		}

		libDirAbs, err = m.PrepareLocalDir(depsDir, lpr.logWriter, true, libsDefVersion, updateIntvl, 0)
		if err != nil {
			if m.Version == "" && libsDefVersion != "latest" {
				// We failed to fetch lib at the default version (mos.version),
				// which is not "latest", and the lib in manifest does not have
				// version specified explicitly. This might happen when some
				// latest app is built with older mos tool.

				serverVersion := libsDefVersion
				v, err := update.GetServerMosVersion(string(update.GetUpdateChannel()))
				if err == nil {
					serverVersion = v.BuildVersion
				}

				ourutil.Freportf(logWriterStderr,
					"WARNING: the lib %q does not have version %s. Resorting to latest, but the build might fail.\n"+
						"It usually happens if you clone the latest version of some example app, and try to build it with the mos tool which is older than the lib (in this case, %q).", name, libsDefVersion, name,
				)

				if serverVersion != version.GetMosVersion() {
					// There is a newer version of the mos tool available, so
					// suggest upgrading.

					ourutil.Freportf(logWriterStderr,
						"There is a newer version of the mos tool available: %s, try to update mos tool (mos update), and build again. "+
							"Alternatively, you can build the version %s of the app (git checkout %s).", serverVersion, libsDefVersion, libsDefVersion,
					)
				} else {
					// Current mos is at the newest released version, so the only
					// alternatives are: build older (released) version of the app,
					// or use latest mos.

					ourutil.Freportf(logWriterStderr,
						"Consider using the version %s of the app (git checkout %s), or using latest mos tool (mos update latest).", libsDefVersion, libsDefVersion,
					)
				}

				// In any case, retry with the latest lib version and cross fingers.

				libsDefVersion = "latest"
				continue
			}
			return "", errors.Annotatef(err, "%s: preparing local copy", name)
		}

		if m.GetType() == build.SWModuleTypeGit && updateIntvl != 0 {
			if newHash, _, err := m.GetRepoVersion(); err == nil && newHash != curHash {
				freportf(logWriter, "%s: Hash is updated: %s -> %s", name, curHash, newHash)
				// The current repo hash has changed after the pull, so we need to
				// vanish binary lib(s) we might have downloaded before
				bLibs, _ := filepath.Glob(moscommon.GetBinaryLibFilePath(moscommon.GetBuildDir(appDir), name, "*", "*"))
				for _, bl := range bLibs {
					if os.Remove(bl) == nil {
						freportf(logWriterStderr, "%s: Removed %s because the repo has been updated", name, bl)
					}
				}
			} else {
				freportf(logWriter, "%s: Hash unchanged at %s (dir %q)", name, curHash, libDirAbs)
			}
		}

		break
	}
	ourutil.Freportf(lpr.logWriter, "%s: Prepared local dir: %q", name, libDirAbs)

	return libDirAbs, nil
}

func (lpr *compProviderReal) GetModuleLocalPath(
	m *build.SWModule, rootAppDir, modulesDefVersion, platform string,
) (string, error) {
	name, err := m.GetName()
	if err != nil {
		return "", errors.Trace(err)
	}

	m.SetCredentials(lpr.bParams.GetCredentialsForHost(m.GetHostName()))

	customLoc, ok := lpr.bParams.CustomModuleLocations[name]
	if ok && !isURL(customLoc) {
		freportf(logWriter, "Using module %q located at %q", name, customLoc)
	}

	if ok {
		m.Location = customLoc
	}

	// Custom module location wasn't provided on the command line, so, we'll
	// use the module name and will clone/pull it if necessary
	freportf(logWriter, "%s: Going to fetch module from %s", name, m.Location)

	appDir, err := getCodeDirAbs()
	if err != nil {
		return "", errors.Trace(err)
	}

	updateIntvl := lpr.bParams.LibsUpdateInterval

	targetDir, err := m.PrepareLocalDir(paths.GetModulesDir(appDir), logWriter, true, modulesDefVersion, updateIntvl, 0)
	if err != nil {
		return "", errors.Annotatef(err, "preparing local copy of the module %q", name)
	}

	return targetDir, nil
}

// }}}

// Thread-safe bytes.Buffer {{{

type threadSafeBuffer struct {
	buf bytes.Buffer
	mtx sync.Mutex
}

func (b *threadSafeBuffer) Write(p []byte) (n int, err error) {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	return b.buf.Write(p)
}

func (b *threadSafeBuffer) Bytes() []byte {
	b.mtx.Lock()
	defer b.mtx.Unlock()

	return b.buf.Bytes()
}

// }}}
