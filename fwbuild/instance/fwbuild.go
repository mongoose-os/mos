/*
 * Copyright (c) 2014-2018 Cesanta Software Limited
 * All rights reserved
 *
 * Licensed under the Apache License, Version 2.0 (the ""License"");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an ""AS IS"" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"cesanta.com/common/go/docker"
	"cesanta.com/common/go/ourglob"
	"cesanta.com/common/go/ourio"
	fwbuildcommon "cesanta.com/fwbuild/common"
	"cesanta.com/fwbuild/common/reqpar"
	"cesanta.com/mos/build"
	"cesanta.com/mos/build/archive"
	moscommon "cesanta.com/mos/common"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flock "github.com/theckman/go-flock"
	yaml "gopkg.in/yaml.v2"
)

var (
	volumesDir = flag.String("volumes-dir", "/var/tmp/fwbuild-volumes", "dir where build volumes are created")
	mosImage   = flag.String("mos-image", "docker.io/mgos/mos:latest",
		"cloud-mos docker image")

	reqParFileName    = flag.String("req-params", "", "Request params filename")
	outputZipFileName = flag.String("output-zip", "", "Output zip filename")

	locks = &locksStruct{
		flockByPath: map[string]*flock.Flock{},
	}

	errBuildFailure = errors.New("build failure")
)

const (
	payloadLimit = 2 * 1024 * 1024

	appsRootName = "apps"

	updateSharedReposInterval = time.Minute * 30

	buildCtxInfoFilename = "build_ctx_info.json"
)

// buildCtxItem represents a file which is present in at least source or
// target. If both are present, and their hashes are equal, then the target
// item should be left intact. If hashes are not equal, then target item should
// be overwritten with the source item. If target is present, but source is
// not, then target should be deleted. And, of course, if source is present,
// but target is not, then source should be copies as a target.
type buildCtxItem struct {
	SrcItem *BuildCtxInfoFile
	TgtItem *BuildCtxInfoFile
}

// updateBuildCtx tries to incrementally update existing build context tgt with
// the newly uploaded sources src. Both tgt and src are paths to dirs.
//
// updateBuildCtx reads build context metadata (BuildCtxInfo) of both source
// and target, and performs the sync appropriately.
func updateBuildCtx(src, tgt string) error {

	// Compute a map of files which are present in at least source or target {{{
	m := map[string]buildCtxItem{}

	srcInfo, err := readBuildCtxInfo(src)
	if err != nil {
		return errors.Trace(err)
	}

	tgtInfo, err := readBuildCtxInfo(tgt)
	if err != nil {
		return errors.Trace(err)
	}

	for k, v := range srcInfo.Files {
		item := m[k]
		item.SrcItem = v
		m[k] = item
	}

	for k, v := range tgtInfo.Files {
		item := m[k]
		item.TgtItem = v
		m[k] = item
	}
	// }}}

	totalCnt := 0
	updatedCnt := 0

	// Compute sorted slice of all paths
	keys := []string{}
	for k, _ := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	// Iterate over all items (which are present in either src or tgt)
	for _, k := range keys {
		v := m[k]
		equal := false
		totalCnt++

		srcItemPath := filepath.Join(src, k)
		tgtItemPath := filepath.Join(tgt, k)

		if v.TgtItem != nil && v.SrcItem != nil && v.TgtItem.Hash == v.SrcItem.Hash {
			equal = true
		}

		if !equal {
			if v.TgtItem == nil {
				glog.Infof("ADD    %q", k)
			} else if v.SrcItem == nil {
				glog.Infof("REMOVE %q", k)
			} else {
				glog.Infof("UPDATE %q", k)
			}
			updatedCnt++
			// Remove the target item, ignoring any error (at least it might not even exist)
			os.RemoveAll(tgtItemPath)

			// If source is present, rename it as a target (or create an empty dir
			// if source is a dir)
			if v.SrcItem != nil {
				if !v.SrcItem.IsDir {
					if err := os.Rename(srcItemPath, tgtItemPath); err != nil {
						return errors.Trace(err)
					}
				} else {
					if err := os.Mkdir(tgtItemPath, 0777); err != nil {
						return errors.Trace(err)
					}
				}
			}
		} else {
			// Items are equal, leaving target intact
			glog.Infof("EQ     %q", k)
		}
	}

	srcInfoFilename := filepath.Join(src, buildCtxInfoFilename)
	tgtInfoFilename := filepath.Join(tgt, buildCtxInfoFilename)

	os.RemoveAll(tgtInfoFilename)
	if err := os.Rename(srcInfoFilename, tgtInfoFilename); err != nil {
		return errors.Trace(err)
	}

	// TODO(dfrank): make sure the new build context info file is in sync with
	// the actual files; if not, return an error, so that the build will be
	// clean. Just to ensure that if there is a bug in syncing, the build will
	// not be affected

	glog.Infof("Files processed: %d, files updated: %d", totalCnt, updatedCnt)

	return nil
}

func readBuildCtxInfo(src string) (*BuildCtxInfo, error) {
	data, err := ioutil.ReadFile(filepath.Join(src, buildCtxInfoFilename))
	if err != nil {
		return nil, errors.Trace(err)
	}

	bctxInfo := BuildCtxInfo{}

	if err := json.Unmarshal(data, &bctxInfo); err != nil {
		return nil, errors.Trace(err)
	}

	return &bctxInfo, nil
}

func saveBuildCtxInfo(src string) error {
	bctxInfo, err := GetBuildCtxInfo(src)
	if err != nil {
		return errors.Trace(err)
	}

	data, err := json.MarshalIndent(bctxInfo, "", "  ")
	if err != nil {
		return errors.Trace(err)
	}

	if err := ioutil.WriteFile(filepath.Join(src, buildCtxInfoFilename), data, 0666); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// buildFirmware expects a ZIP file in sources and a user/group name in account
// it will unpack the sources in a per-account directory, parse the mg-app yaml file
// in order to figure out the architecture and invokes the docker build image for that
// container.
//
// The dir hierarchy looks as follows:
//
//    *volumesDir
//    └── apps
//        └── app_name
//            └── arch_name
//                └── build_contexts
//                    ├── build_ctx_xxxxxxxx
//                    │   ├── deps
//                    │   │   └── ... managed by mos
//                    │   ├── mos.yml
//                    │   ├── src
//                    │   └── ...other source files...
//                    ├── build_ctx_yyyyyyyy
//                    └── ...contexts...
//
// The build_ctx_xxxxx dir is created for each "build context". Build context
// just contains uploaded source files, context metadata (see BuildCtxInfo),
// and build artifacts.
//
// buildFirmware runs mos in the container docker.io/mgos/mos, which in
// turn will spawn one more container which will perform the actual build. The
// build will be run as an unprivileged user, thus we have to ensure that the
// output dir can be written to an arbitrary user that actually runs within
// another docker container (we don't know the uid).
//
// In order to spawn a docker container, this binary has to have access to the docker daemon
// socket and the volume paths it sees must be the same as the ones seen by the docker deamon.
// In practice that means if you run this in a docker container you have to bind:
//
//  - /tmp/fwbuild-volumes:/tmp/fwbuild-volumes
//  - /var/run/docker.sock:/var/run/docker.sock
func buildFirmware() error {
	glog.Infof("building firwmare")

	if *reqParFileName == "" {
		return errors.Errorf("--req-params is missing")
	}

	if *outputZipFileName == "" {
		return errors.Errorf("--output-zip is missing")
	}

	// Read request params
	reqParData, err := ioutil.ReadFile(*reqParFileName)
	if err != nil {
		return errors.Trace(err)
	}

	fmt.Println(string(reqParData))

	var reqPar reqpar.RequestParams

	if err := json.Unmarshal(reqParData, &reqPar); err != nil {
		return errors.Trace(err)
	}

	sourcesFilename := reqPar.FormFileName(moscommon.FormSourcesZipName)
	if sourcesFilename == "" {
		return errors.Errorf("%s is missing from the request", moscommon.FormSourcesZipName)
	}

	sources, err := ioutil.ReadFile(sourcesFilename)
	if err != nil {
		return errors.Trace(err)
	}
	glog.Infof("body size: %d", len(sources))

	w, err := os.Create(*outputZipFileName)
	if err != nil {
		return errors.Trace(err)
	}

	// Log build stat of the latest build
	buildStatData := reqPar.FormValue(moscommon.FormBuildStatName)
	if buildStatData != "" {
		fmt.Println("Build stat of the latest build:")
		fmt.Println(buildStatData)
	} else {
		fmt.Println("No stat of the latest build received")
	}

	buildCtxName := reqPar.FormValue(moscommon.FormBuildCtxName)
	clean := reqPar.FormValue(moscommon.FormCleanName) != ""

	preferPrebuildLibs := reqPar.FormValue(moscommon.FormPreferPrebuildLibsName) == "1"

	// we need to unpack sources to temp dir first, because the actual
	// destination depends on the app name which is set into the manifest
	tmpCodeDir, err := ioutil.TempDir(*volumesDir, "tmp_src_")
	if err != nil {
		return errors.Trace(err)
	}
	defer os.RemoveAll(tmpCodeDir)

	// unzip sources
	bytesReader := bytes.NewReader(sources)
	if err := archive.UnzipInto(bytesReader, bytesReader.Size(), tmpCodeDir, 0); err != nil {
		return errors.Trace(err)
	}

	// Calculate newly received build context info
	if err := saveBuildCtxInfo(tmpCodeDir); err != nil {
		return errors.Trace(err)
	}

	manifestPath := moscommon.GetManifestFilePath(tmpCodeDir)

	// read the manifest to figure out which arch we're building for.
	manifestSrc, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return errors.Trace(err)
	}

	var manifest build.FWAppManifest
	if err := yaml.Unmarshal(manifestSrc, &manifest); err != nil {
		return errors.Trace(err)
	}

	appsRoot := filepath.Join(*volumesDir, appsRootName)
	appRoot := filepath.Join(appsRoot, manifest.Name)
	appArchRoot := filepath.Join(appRoot, manifest.Platform)
	if manifest.Platform == "" && manifest.ArchOld != "" {
		appArchRoot = filepath.Join(appRoot, manifest.ArchOld)
	}
	appBuildCtxRoot := filepath.Join(appArchRoot, "build_contexts")

	if err := os.MkdirAll(appBuildCtxRoot, 0777); err != nil {
		return errors.Trace(err)
	}

	// Make sure buildCtxName does not contain illegal chars

	if !regexp.MustCompile("^[a-zA-Z0-9_]*$").Match([]byte(buildCtxName)) {
		glog.Warningf("Illegal build context name %q, cleaning up", buildCtxName)
		buildCtxName = ""
	}

	// Figure codeDir {{{
	codeDir := ""

	if buildCtxName != "" {
		// build context name was provided; let's see if it exists
		codeDir = filepath.Join(appBuildCtxRoot, buildCtxName)
		if _, err := os.Stat(codeDir); err != nil {
			if !os.IsNotExist(err) {
				return errors.Trace(err)
			}

			// The given build context does not exist; will have to create a new one
			// (with the autogenerated name)
			codeDir = ""
		}
	}

	if clean {
		if codeDir != "" {
			glog.Infof("Delete old build context %s", codeDir)
			os.RemoveAll(codeDir)
			os.RemoveAll(getFlockNameByPath(codeDir))
		}
		buildCtxName = ""
		codeDir = ""
	}

	if codeDir == "" {
		glog.Infof("Create a new build context")
		codeDir, err = ioutil.TempDir(appBuildCtxRoot, "build_ctx_")
		if err != nil {
			return errors.Trace(err)
		}
	}
	// }}}

	fl := locks.getFlockByPath(codeDir)
	fl.Lock()
	defer fl.Unlock()

	glog.Infof("=== Start building in %q", codeDir)
	defer func() {
		glog.Infof("=== Done building in %q", codeDir)
	}()

	// Remember the actual build context name
	_, buildCtxName = filepath.Split(codeDir)

	if !clean {
		if err := updateBuildCtx(tmpCodeDir, codeDir); err != nil {
			glog.Infof("Couldn't update build context incrementally: %s, resort to clean build", err)
			clean = true
		}
	}

	// If the build is clean, just vanish existing codeDir (if any), and rename
	// the uploaded sources to codeDir
	if clean {
		os.RemoveAll(codeDir)
		if err := os.Rename(tmpCodeDir, codeDir); err != nil {
			return errors.Trace(err)
		}
	}

	if err := os.Chmod(codeDir, 0777); err != nil { // compiler runs as a user
		return errors.Trace(err)
	}

	// Temp directory for mos
	codeTmpDir := filepath.Join(codeDir, "tmp")
	if err := os.MkdirAll(codeTmpDir, 0755); err != nil {
		return errors.Trace(err)
	}

	// Write out build params
	buildParams := reqPar.FormValue(moscommon.FormBuildParamsName)
	if buildParams == "" {
		return errors.Errorf("no build params")
	}
	bpFile := filepath.Join(codeDir, "build_params.yml")
	if err := ioutil.WriteFile(bpFile, []byte(buildParams), 0644); err != nil {
		return errors.Annotatef(err, "failed to write build params")
	}

	var buildOutput bytes.Buffer
	out := io.MultiWriter(&buildOutput, os.Stderr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run cloud-mos docker container which will do the build {{{
	success := true
	err = docker.Run(
		ctx, *mosImage, out,
		// Mgos container should be able to spawn other containers
		// (read about the "sibling containers" "approach:
		// https://jpetazzo.github.io/2015/09/03/do-not-use-docker-in-docker-for-ci/)
		docker.Bind("/var/run/docker.sock", "/var/run/docker.sock", "rw"),
		// Mount code dir to the same location, because the location should
		// actually be the same across the host and all the containers which need
		// to bind it to the "sibling" containers.
		//
		// Note that we mount appRoot instead of codeDir, since appRoot contains
		// shared repos of app-dependent modules, and private clones in codeDir
		// reference them.
		docker.Bind(appRoot, appRoot, "rw"),
		// We also need to bind the shared mongoose-os repo, because the one
		// in the build directory references it. We mount it in read-only mode.
		docker.Cmd([]string{
			"build", "-C", codeDir, "--local", "--verbose",
			"--migrate=false",
			"--save-build-stat=false",
			fmt.Sprintf("--build-params=%s", bpFile),
			"--temp-dir", codeTmpDir,
			fmt.Sprintf("--prefer-prebuilt-libs=%v", preferPrebuildLibs),
		}),
	)
	if err != nil {
		if _, ok := errors.Cause(err).(*docker.ExitError); ok {
			success = false
		} else {
			return errors.Trace(err)
		}
	}
	// }}}

	buildDir := moscommon.GetBuildDir(codeDir)

	if !success {
		// In case of build error, we also want to capture the mos output which
		// is not necessarily included into the build.log: e.g. if there's no
		// arch in either mos.yml or in CLI.
		//
		// Note that we don't just write to build.log because mos tool does that.
		ioutil.WriteFile(moscommon.GetBuildLogFilePath(buildDir), buildOutput.Bytes(), 0666)
	}

	// Save build context name
	if err := ioutil.WriteFile(
		moscommon.GetBuildCtxFilePath(buildDir), []byte(buildCtxName), 0666,
	); err != nil {
		return errors.Trace(err)
	}

	// Pack build directory ignoring build/objs/* except build/objs/fw.elf
	matcher := ourglob.PatItems{
		{"build/objs/fw.elf", true},
		{"build/objs/*", false},
		{"*", true},
	}
	var archiveData bytes.Buffer
	if err := ourio.Archive(
		buildDir,
		&archiveData,
		func(archivePath string) bool {
			match, err := matcher.Match(archivePath)
			if err != nil {
				// Error can only be returned in the case of malformed pattern,
				// so it should never happen in production
				panic(err.Error())
			}

			return match
		},
	); err != nil {
		return errors.Trace(err)
	}

	if _, err := w.Write(archiveData.Bytes()); err != nil {
		return errors.Trace(err)
	}

	if success {
		return nil
	} else {
		return errBuildFailure
	}
}

func sendData(w http.ResponseWriter, status int, data []byte) error {
	// otherwise it defaults to chunked encoding which isn't supported by the
	// mongoose reverse proxy yet.
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func usage() {
	fmt.Printf("Fwbuilder. Usage: %s [flags] <action>\n", os.Args[0])
	fmt.Printf("Action can be: %q\n", "build")
}

func main() {
	flag.Parse()

	if err := os.MkdirAll(*volumesDir, 0775); err != nil {
		glog.Fatal(err)
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Missing action argument")
		usage()
		os.Exit(1)
	}

	action := args[0]

	switch action {
	case "build":
		if err := buildFirmware(); err != nil {
			if errors.Cause(err) == errBuildFailure {
				os.Exit(fwbuildcommon.FwbuildExitCodeBuildFailed)
			}
			fmt.Println(err)
			os.Exit(1)
		}
	default:
		fmt.Println("Invalid action")
		usage()
		os.Exit(1)
	}
}

func isBuildVarAllowed(name string) bool {
	return strings.HasPrefix(name, "MG_ENABLE_") ||
		strings.HasPrefix(name, "APP_")
}

// locksStruct is needed to maintain mutexes on a per-path basis; see
// getFlockByPath()
type locksStruct struct {
	flockByPath map[string]*flock.Flock
}

// getFlockByPath takes a path and returns a pointer to a mutex for that path.
// When called first time for some particular path, the newly created mutex is
// saved into the map and returned.
func (l *locksStruct) getFlockByPath(path string) *flock.Flock {
	if fl, ok := l.flockByPath[path]; ok {
		return fl
	} else {
		fl := flock.NewFlock(getFlockNameByPath(path))
		l.flockByPath[path] = fl
		return fl
	}
}

func getFlockNameByPath(path string) string {
	return fmt.Sprint(path, ".fwbuild-lock")
}
