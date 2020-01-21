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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"context"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/build"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/interpreter"
	"github.com/mongoose-os/mos/cli/manifest_parser"
	"github.com/mongoose-os/mos/cli/mosgit"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/common/multierror"
	"github.com/mongoose-os/mos/common/ourio"
	yaml "gopkg.in/yaml.v2"
)

func buildLocal(ctx context.Context, bParams *buildParams) error {
	if isInDockerToolbox() {
		freportf(logWriterStderr, "Docker Toolbox detected")
	}

	buildDir := moscommon.GetBuildDir(projectDir)

	buildErr := buildLocal2(ctx, bParams, *cleanBuildFlag)

	if !*verbose && buildErr != nil {
		log, err := os.Open(moscommon.GetBuildLogFilePath(buildDir))
		if err != nil {
			glog.Errorf("can't read build log: %s", err)
		} else {
			io.Copy(os.Stdout, log)
		}
	}

	return buildErr
}

func generateCflags(cflags []string, cdefs map[string]string) string {
	kk := []string{}
	for k, _ := range cdefs {
		kk = append(kk, k)
	}
	sort.Strings(kk)
	for _, k := range kk {
		v := cdefs[k]
		cflags = append(cflags, fmt.Sprintf("-D%s=%s", k, v))
	}

	return strings.Join(append(cflags), " ")
}

func buildLocal2(ctx context.Context, bParams *buildParams, clean bool) (err error) {
	gitinst := mosgit.NewOurGit()

	buildDir := moscommon.GetBuildDir(projectDir)

	buildDirAbs, err := filepath.Abs(buildDir)
	if err != nil {
		return errors.Trace(err)
	}

	genDir := moscommon.GetGeneratedFilesDir(buildDirAbs)

	fwDir := moscommon.GetFirmwareDir(buildDirAbs)
	fwDirDocker := ourutil.GetPathForDocker(fwDir)

	objsDir := moscommon.GetObjectDir(buildDirAbs)
	objsDirDocker := ourutil.GetPathForDocker(objsDir)

	fwFilename := moscommon.GetFirmwareZipFilePath(buildDir)

	// Perform cleanup before the build {{{
	if clean {
		// Cleanup build dir, but leave build log intact, because we're already
		// writing to it.
		if err := ourio.RemoveFromDir(buildDir, []string{moscommon.GetBuildLogFilePath("")}); err != nil {
			return errors.Trace(err)
		}
	} else {
		// This is not going to be a clean build, but we should still remove fw.zip
		// (ignoring any possible errors)
		os.Remove(fwFilename)
	}
	// }}}

	// Prepare gen dir
	if err := os.MkdirAll(genDir, 0777); err != nil {
		return errors.Trace(err)
	}

	compProvider := compProviderReal{
		bParams:   bParams,
		logWriter: logWriter,
	}

	interp := interpreter.NewInterpreter(newMosVars())

	appDir, err := getCodeDirAbs()
	if err != nil {
		return errors.Trace(err)
	}

	libsUpdateIntvl := *libsUpdateInterval
	if *noLibsUpdate {
		libsUpdateIntvl = 0
	}
	manifest, fp, err := manifest_parser.ReadManifestFinal(
		appDir, &bParams.ManifestAdjustments, logWriter, interp,
		&manifest_parser.ReadManifestCallbacks{ComponentProvider: &compProvider},
		true /* requireArch */, *preferPrebuiltLibs, libsUpdateIntvl)
	if err != nil {
		return errors.Trace(err)
	}

	// Write final manifest to build dir
	manifestUpdated, err := ourio.WriteYAMLFileIfDifferent(moscommon.GetMosFinalFilePath(buildDirAbs), manifest, 0666)
	if err != nil {
		return errors.Trace(err)
	}
	// Force clean rebuild if manifest was updated
	if manifestUpdated && !clean {
		freportf(logWriter, "== Manifest has changed, forcing a clean rebuild...")
		return buildLocal2(ctx, bParams, true /* clean */)
	}

	switch manifest.Type {
	case build.AppTypeApp:
		// Fine
	case build.AppTypeLib:
		bParams.BuildTarget = moscommon.GetOrigLibArchiveFilePath(buildDir, manifest.Platform)
		if manifest.Platform == "esp32" {
			*buildCmdExtra = append(*buildCmdExtra, "MGOS_MAIN_COMPONENT=moslib")
		}
	default:
		return errors.Errorf("invalid project type: %q", manifest.Type)
	}

	curConfSchemaFName := ""
	// If config schema is provided in manifest, generate a yaml file suitable
	// for `APP_CONF_SCHEMA`
	if manifest.ConfigSchema != nil && len(manifest.ConfigSchema) > 0 {
		var err error
		curConfSchemaFName = moscommon.GetConfSchemaFilePath(buildDirAbs)

		confSchemaData, err := yaml.Marshal(manifest.ConfigSchema)
		if err != nil {
			return errors.Trace(err)
		}

		if err = ioutil.WriteFile(curConfSchemaFName, confSchemaData, 0666); err != nil {
			return errors.Trace(err)
		}

		// The modification time of conf schema file should be set to that of
		// the manifest itself, so that make handles dependencies correctly.
		if err := os.Chtimes(curConfSchemaFName, fp.MTime, fp.MTime); err != nil {
			return errors.Trace(err)
		}
	}

	// Check if the app supports the given arch
	found := false
	for _, v := range manifest.Platforms {
		if v == manifest.Platform {
			found = true
			break
		}
	}

	if !found {
		if !bParams.NoPlatformCheck {
			return errors.Errorf(
				"can't build for the platform %s; supported platforms are: %v "+
					"(use --no-platform-check to override)",
				manifest.Platform, manifest.Platforms)
		}
	}

	appSources, err := absPathSlice(manifest.Sources)
	if err != nil {
		return errors.Trace(err)
	}

	appIncludes, err := absPathSlice(manifest.Includes)
	if err != nil {
		return errors.Trace(err)
	}

	appFSFiles, err := absPathSlice(manifest.Filesystem)
	if err != nil {
		return errors.Trace(err)
	}

	appBinLibs, err := absPathSlice(manifest.BinaryLibs)
	if err != nil {
		return errors.Trace(err)
	}

	appSourceDirs, err := absPathSlice(fp.AppSourceDirs)
	if err != nil {
		return errors.Trace(err)
	}

	appFSDirs, err := absPathSlice(fp.AppFSDirs)
	if err != nil {
		return errors.Trace(err)
	}

	appBinLibDirs, err := absPathSlice(fp.AppBinLibDirs)
	if err != nil {
		return errors.Trace(err)
	}

	freportf(logWriter, "Sources: %v", appSources)
	freportf(logWriter, "Include dirs: %v", appIncludes)
	freportf(logWriter, "Binary libs: %v", appBinLibs)

	freportf(logWriter, "Building...")

	appName, err := fixupAppName(manifest.Name)
	if err != nil {
		return errors.Trace(err)
	}

	var errs error
	for k, v := range map[string]string{
		"PLATFORM":       manifest.Platform,
		"BUILD_DIR":      objsDirDocker,
		"FW_DIR":         fwDirDocker,
		"GEN_DIR":        ourutil.GetPathForDocker(genDir),
		"FS_STAGING_DIR": ourutil.GetPathForDocker(moscommon.GetFilesystemStagingDir(buildDirAbs)),
		"APP":            appName,
		"APP_VERSION":    manifest.Version,
		"APP_SOURCES":    strings.Join(getPathsForDocker(appSources), " "),
		"APP_INCLUDES":   strings.Join(getPathsForDocker(appIncludes), " "),
		"APP_FS_FILES":   strings.Join(getPathsForDocker(appFSFiles), " "),
		"APP_BIN_LIBS":   strings.Join(getPathsForDocker(appBinLibs), " "),
		"FFI_SYMBOLS":    strings.Join(manifest.FFISymbols, " "),
		"APP_CFLAGS":     generateCflags(manifest.CFlags, manifest.CDefs),
		"APP_CXXFLAGS":   generateCflags(manifest.CXXFlags, manifest.CDefs),
		"MANIFEST_FINAL": ourutil.GetPathForDocker(moscommon.GetMosFinalFilePath(buildDirAbs)),
	} {
		err := addBuildVar(manifest, k, v)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	if errs != nil {
		return errors.Trace(errs)
	}

	// If config schema file was generated, set APP_CONF_SCHEMA appropriately.
	if curConfSchemaFName != "" {
		if err := addBuildVar(manifest, "APP_CONF_SCHEMA", ourutil.GetPathForDocker(curConfSchemaFName)); err != nil {
			return errors.Trace(err)
		}
	}

	appPath, err := getCodeDirAbs()
	if err != nil {
		return errors.Trace(err)
	}

	makeFilePath := moscommon.GetPlatformMakefilePath(fp.MosDirEffective, manifest.Platform)
	makeVarsFileSupported := false
	if data, err := ioutil.ReadFile(makeFilePath); err == nil {
		makeVarsFileSupported = bytes.Contains(data, []byte("MGOS_VARS_FILE"))
	}

	appSubdir := ""

	// Invoke actual build (docker or make) {{{
	if os.Getenv("MGOS_SDK_REVISION") == "" && os.Getenv("MIOT_SDK_REVISION") == "" {
		// We're outside of the docker container, so invoke docker

		var dockerAppPath, dockerMgosPath string

		dockerRunArgs := []string{"--rm", "-i"}

		gitToplevelDir, _ := gitinst.GetToplevelDir(appPath)

		if *flags.BuildDockerNoMounts {
			// User wants no mounts, just use paths directly.
			dockerAppPath = appPath
			dockerMgosPath = fp.MosDirEffective
			if len(*flags.BuildDockerExtra) == 0 {
				glog.Warning("--build-docker-no-mounts specified but no --build-docker-extra " +
					"arguments given, build will most likely fail.")
			}
		} else {
			// Generate mountpoint args {{{
			mp := mountPoints{}

			dockerAppPath = "/app"
			dockerMgosPath = "/mongoose-os"

			appMountPath := ""
			if gitToplevelDir == "" {
				// We're outside of any git repository: will just mount the application
				// path
				appMountPath = appPath
				appSubdir = ""
			} else {
				// We're inside some git repo: will mount the root of this repo, and
				// remember the app's subdir inside it.
				appMountPath = gitToplevelDir
				appSubdir = appPath[len(gitToplevelDir):]
			}

			// Note about mounts: we mount repo to a stable path (/app) as well as the
			// original path outside the container, whatever it may be, so that absolute
			// path references continue to work (e.g. Git submodules are known to use
			// abs. paths).
			mp.addMountPoint(appMountPath, dockerAppPath)
			mp.addMountPoint(fp.MosDirEffective, dockerMgosPath)
			mp.addMountPoint(fp.MosDirEffective, ourutil.GetPathForDocker(fp.MosDirEffective))

			manifest.BuildVars["MGOS_PATH"] = ourutil.GetPathForDocker(fp.MosDirEffective)

			// Mount build dir
			mp.addMountPoint(buildDirAbs, ourutil.GetPathForDocker(buildDirAbs))

			// Mount all dirs with source files
			for _, d := range appSourceDirs {
				mp.addMountPoint(d, ourutil.GetPathForDocker(d))
			}

			// Mount all include paths
			for _, d := range appIncludes {
				mp.addMountPoint(d, ourutil.GetPathForDocker(d))
			}

			// Mount all dirs with filesystem files
			for _, d := range appFSDirs {
				mp.addMountPoint(d, ourutil.GetPathForDocker(d))
			}

			// Mount all dirs with binary libs
			for _, d := range appBinLibDirs {
				mp.addMountPoint(d, ourutil.GetPathForDocker(d))
			}

			// If generated config schema file is present, mount its dir as well
			if curConfSchemaFName != "" {
				d := filepath.Dir(curConfSchemaFName)
				mp.addMountPoint(d, ourutil.GetPathForDocker(d))
			}

			for containerPath, hostPath := range mp {
				dockerRunArgs = append(dockerRunArgs, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
			}
			// }}}
		}

		// On Windows and Mac, run container as root since volume sharing on those
		// OSes doesn't play nice with unprivileged user.
		//
		// On other OSes, run it as the current user.
		if runtime.GOOS == "linux" {
			// Unfortunately, user.Current() sometimes panics when the mos binary is
			// built statically, so we have to do the trick with "id -u". Since this
			// code runs on Linux only, this workaround does the trick.
			var data bytes.Buffer
			cmd := exec.Command("id", "-u")
			cmd.Stdout = &data
			if err := cmd.Run(); err != nil {
				return errors.Trace(err)
			}
			sdata := data.String()
			userID := sdata[:len(sdata)-1]

			dockerRunArgs = append(
				dockerRunArgs, "--user", fmt.Sprintf("%s:%s", userID, userID),
			)
		}

		// Add extra docker args
		dockerRunArgs = append(dockerRunArgs, (*flags.BuildDockerExtra)...)

		sdkVersionFile := moscommon.GetSdkVersionFile(fp.MosDirEffective, manifest.Platform)

		buildImage := *flags.BuildImage

		if buildImage == "" {
			// Get build image name and tag from the repo.
			sdkVersionBytes, err := ioutil.ReadFile(sdkVersionFile)
			if err != nil {
				return errors.Annotatef(err, "failed to read sdk version file %q", sdkVersionFile)
			}

			buildImage = strings.TrimSpace(string(sdkVersionBytes))
		}

		manifest.BuildVars["MGOS_PATH"] = dockerMgosPath

		dockerRunArgs = append(dockerRunArgs, buildImage)

		makeArgs, err := getMakeArgs(
			filepath.ToSlash(fmt.Sprintf("%s%s", dockerAppPath, appSubdir)),
			makeFilePath,
			bParams.BuildTarget,
			buildDirAbs,
			manifest,
			makeVarsFileSupported,
		)
		if err != nil {
			return errors.Trace(err)
		}

		dockerRunArgs = append(dockerRunArgs,
			"/bin/bash", "-c", "nice make '"+strings.Join(makeArgs, "' '")+"'",
		)

		if err := runDockerBuild(dockerRunArgs); err != nil {
			return errors.Trace(err)
		}
		if *buildDryRunFlag {
			return nil
		}
	} else {
		// We're already inside of the docker container, so invoke make directly

		manifest.BuildVars["MGOS_PATH"] = fp.MosDirEffective

		makeArgs, err := getMakeArgs(
			appPath,
			makeFilePath,
			bParams.BuildTarget,
			buildDirAbs,
			manifest,
			makeVarsFileSupported,
		)
		if err != nil {
			return errors.Trace(err)
		}

		freportf(logWriter, "Make arguments: %s", strings.Join(makeArgs, " "))

		if *buildDryRunFlag {
			return nil
		}

		cmd := exec.Command("make", makeArgs...)
		err = runCmd(cmd, logWriter)
		if err != nil {
			return errors.Trace(err)
		}
	}
	// }}}

	if bParams.BuildTarget == moscommon.BuildTargetDefault {
		// We were building a firmware, so perform the required actions with moving
		// firmware around, etc.

		// Copy firmware to build/fw.zip
		err = ourio.LinkOrCopyFile(
			filepath.Join(fwDir, fmt.Sprintf("%s-%s-last.zip", appName, manifest.Platform)),
			fwFilename,
		)
		if err != nil {
			return errors.Trace(err)
		}
	} else if p := moscommon.GetOrigLibArchiveFilePath(buildDir, manifest.Platform); bParams.BuildTarget == p {
		// Copy lib to build/lib.a
		err = ourio.LinkOrCopyFile(
			p, moscommon.GetLibArchiveFilePath(buildDir),
		)
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func isInDockerToolbox() bool {
	return os.Getenv("DOCKER_HOST") != ""
}

func getMakeArgs(dir, makeFilePath, target, buildDirAbs string, manifest *build.FWAppManifest, makeVarsFileSupported bool) ([]string, error) {
	j := *flags.BuildParalellism
	if j == 0 {
		j = runtime.NumCPU()
	}

	// If target contains a slash, assume it's a path name, and absolutize it
	// (that's a requirement because in makefile paths are absolutized).
	// Actually, all file targets are going to begin with "build/", so this check
	// is reliable.
	if strings.Contains(target, "/") {
		var err error
		target, err = filepath.Abs(target)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	makeArgs := []string{
		"-j", fmt.Sprintf("%d", j),
		"-C", dir,
		"-f", ourutil.GetPathForDocker(makeFilePath),
		target,
	}

	// Write make vars file.
	if err := ioutil.WriteFile(
		moscommon.GetMakeVarsFilePath(buildDirAbs),
		[]byte(strings.Join(getMakeVars(manifest.BuildVars, true /* escHash */), "\n")),
		0644); err != nil {
		return nil, errors.Annotatef(err, "writing vars file")
	}

	if makeVarsFileSupported {
		makeArgs = append(makeArgs, fmt.Sprintf(
			"MGOS_VARS_FILE=%s",
			ourutil.GetPathForDocker(moscommon.GetMakeVarsFilePath(buildDirAbs))))
	} else {
		makeArgs = append(makeArgs, getMakeVars(manifest.BuildVars, false /* escHash */)...)
	}
	// Add extra make args
	if buildCmdExtra != nil {
		makeArgs = append(makeArgs, (*buildCmdExtra)...)
	}

	return makeArgs, nil
}

func getMakeVars(vars map[string]string, escHash bool) []string {
	kk := []string{}
	for k, _ := range vars {
		kk = append(kk, k)
	}
	sort.Strings(kk)
	vv := []string{}
	for _, k := range kk {
		v := vars[k]
		v = strings.Replace(v, "\r", " ", -1)
		v = strings.Replace(v, "\n", " ", -1)
		if escHash {
			v = strings.Replace(v, "#", `\#`, -1)
		}
		vv = append(vv, fmt.Sprintf("%s=%s", k, v))
	}
	return vv
}

type mountPoints map[string]string

// addMountPoint adds a mount point from given hostPath to containerPath. If
// something is already mounted to the given containerPath, then it's compared
// to the new hostPath value; if they are not equal, an error is returned.
func (mp mountPoints) addMountPoint(hostPath, containerPath string) error {
	// Do not mount non-existent paths. This can happen for auto-generated paths
	// such as src/${platform} where no platform-specific sources exist.
	if _, err := os.Stat(hostPath); err != nil {
		return nil
	}

	// Docker Toolbox hack: in docker toolbox on windows, the actual host paths
	// like C:\foo\bar don't work, this path becomes /c/foo/bar.
	if isInDockerToolbox() {
		hostPath = ourutil.GetPathForDocker(hostPath)
	}

	freportf(logWriter, "mount from %q to %q", hostPath, containerPath)
	if v, ok := mp[containerPath]; ok {
		if hostPath != v {
			return errors.Errorf("adding mount point from %q to %q, but it already mounted from %q", hostPath, containerPath, v)
		}
		// Mount point already exists and is right
		return nil
	}
	mp[containerPath] = hostPath

	return nil
}

// addBuildVar adds a given build variable to manifest.BuildVars, but if the
// variable already exists, returns an error (modulo some exceptions, which
// result in a warning instead)
func addBuildVar(manifest *build.FWAppManifest, name, value string) error {
	if _, ok := manifest.BuildVars[name]; ok {
		return errors.Errorf(
			"Build variable %q should not be given in %q "+
				"since it's set by the mos tool automatically",
			name, moscommon.GetManifestFilePath(""),
		)
	}
	manifest.BuildVars[name] = value
	return nil
}

// getPathsForDocker calls ourutil.GetPathForDocker for each paths in the slice,
// and returns modified slice
func getPathsForDocker(p []string) []string {
	ret := make([]string, len(p))
	for i, v := range p {
		ret[i] = ourutil.GetPathForDocker(v)
	}
	return ret
}

func runDockerBuild(dockerRunArgs []string) error {
	containerName := fmt.Sprintf(
		"mos_build_%s_%d", time.Now().Format("2006-01-02T15-04-05-00"), rand.Int(),
	)

	dockerArgs := append(
		[]string{"run", "--name", containerName}, dockerRunArgs...,
	)

	freportf(logWriter, "Docker arguments: %s", strings.Join(dockerArgs, " "))

	if *buildDryRunFlag {
		return nil
	}

	// When make runs with -j and we interrupt the container with Ctrl+C, make
	// becomes a runaway process eating 100% of one CPU core. So far we failed
	// to fix it properly, so the workaround is to kill the container on the
	// reception of SIGINT or SIGTERM.
	signals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	sigCh := make(chan os.Signal, 1)

	// Signal handler goroutine: on SIGINT and SIGTERM it will kill the container
	// and exit(1). When the sigCh is closed, goroutine returns.
	go func() {
		if _, ok := <-sigCh; !ok {
			return
		}

		freportf(logWriterStderr, "\nCleaning up the container %q...", containerName)
		cmd := exec.Command("docker", "kill", containerName)
		cmd.Run()

		os.Exit(1)
	}()

	signal.Notify(sigCh, signals...)
	defer func() {
		// Unsubscribe from the signals and close the channel so that the signal
		// handler goroutine is properly cleaned up
		signal.Reset(signals...)
		close(sigCh)
	}()

	cmd := exec.Command("docker", dockerArgs...)
	if err := runCmd(cmd, logWriter); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// runCmd runs given command and redirects its output to the given log file.
// if --verbose flag is set, then the output also goes to the stdout.
func runCmd(cmd *exec.Cmd, logWriter io.Writer) error {
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	err := cmd.Run()
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
