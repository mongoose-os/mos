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
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"
	yaml "gopkg.in/yaml.v2"

	"github.com/mongoose-os/mos/cli/build"
	"github.com/mongoose-os/mos/cli/build/archive"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/common/paths"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/interpreter"
	"github.com/mongoose-os/mos/cli/manifest_parser"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/common/ourfilepath"
	"github.com/mongoose-os/mos/common/ourio"
	"github.com/mongoose-os/mos/version"
)

const (
	depsDir      = "deps"
	localLibsDir = "local_libs"
)

func buildRemote(bParams *buildParams) error {
	appDir, err := getCodeDirAbs()
	if err != nil {
		return errors.Trace(err)
	}

	buildDir := moscommon.GetBuildDir(projectDir)

	// We'll need to amend the sources significantly with all libs, so copy them
	// to temporary dir first
	appStagingDir, err := ioutil.TempDir(paths.TmpDir, "tmp_mos_src_")
	if err != nil {
		return errors.Trace(err)
	}
	if !*flags.KeepTempFiles {
		defer os.RemoveAll(appStagingDir)
	}
	if *verbose {
		ourutil.Reportf("Using %s as staging dir", appStagingDir)
	}

	// Since we're going to copy sources to the temp dir, make sure that nobody
	// else can read them
	if err := os.Chmod(appStagingDir, 0700); err != nil {
		return errors.Trace(err)
	}

	if err := ourio.CopyDir(appDir, appStagingDir, []string{"build", ".git"}); err != nil {
		return errors.Trace(err)
	}

	// Copy CustomLibLocations and CustomModuleLocations to deps
	for n, libDir := range bParams.CustomLibLocations {
		libDirStaging := filepath.Join(appStagingDir, depsDir, n)
		if *verbose {
			ourutil.Reportf("Copying %s", libDir)
		}
		if err := ourio.CopyDir(libDir, libDirStaging, []string{".git"}); err != nil {
			return errors.Annotatef(err, "failed to copy lib %s", n)
		}
	}
	bParams.CustomLibLocations = nil
	for n, moduleDir := range bParams.CustomModuleLocations {
		moduleDirStaging := filepath.Join(appStagingDir, depsDir, "modules", n)
		if *verbose {
			ourutil.Reportf("Copying %s", moduleDir)
		}
		if err := ourio.CopyDir(moduleDir, moduleDirStaging, []string{".git"}); err != nil {
			return errors.Annotatef(err, "failed to copy module %s", n)
		}
	}
	bParams.CustomModuleLocations = nil

	interp := interpreter.NewInterpreter(newMosVars())

	manifest, _, err := manifest_parser.ReadManifest(appStagingDir, &bParams.ManifestAdjustments, interp)
	if err != nil {
		return errors.Trace(err)
	}

	if manifest.Platform == "" {
		return errors.Errorf("--platform must be specified or mos.yml should contain a platform key")
	}

	// Set the mos.platform variable
	interp.MVars.SetVar(interpreter.GetMVarNameMosPlatform(), manifest.Platform)

	// We still need to expand some conds we have so far, at least to ensure that
	// manifest.Sources contain all the app's sources we need to build, so that
	// they will be whitelisted (see whitelisting logic below) and thus uploaded
	// to the remote builder.
	if err := manifest_parser.ExpandManifestConds(manifest, manifest, interp, true); err != nil {
		return errors.Trace(err)
	}

	switch manifest.Type {
	case build.AppTypeApp:
		// Fine
	case build.AppTypeLib:
		bParams.BuildTarget = moscommon.GetOrigLibArchiveFilePath(buildDir, manifest.Platform)
	default:
		return errors.Errorf("invalid project type: %q", manifest.Type)
	}

	// Copy all external code (which is outside of the appDir) under appStagingDir {{{
	if err := copyExternalCodeAll(&manifest.Sources, appDir, appStagingDir); err != nil {
		return errors.Trace(err)
	}

	if err := copyExternalCodeAll(&manifest.Includes, appDir, appStagingDir); err != nil {
		return errors.Trace(err)
	}

	if err := copyExternalCodeAll(&manifest.Filesystem, appDir, appStagingDir); err != nil {
		return errors.Trace(err)
	}

	if err := copyExternalCodeAll(&manifest.BinaryLibs, appDir, appStagingDir); err != nil {
		return errors.Trace(err)
	}
	// }}}

	manifest.Name, err = fixupAppName(manifest.Name)
	if err != nil {
		return errors.Trace(err)
	}

	// For all handled libs, fixup paths if local separator is different from
	// the Linux separator (because remote builder runs on linux)
	if filepath.Separator != '/' {
		for k, lh := range manifest.LibsHandled {
			manifest.LibsHandled[k].Path = strings.Replace(
				lh.Path, string(filepath.Separator), "/", -1,
			)
		}
	}

	// Write manifest yaml
	manifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		return errors.Trace(err)
	}

	err = ioutil.WriteFile(
		moscommon.GetManifestFilePath(appStagingDir),
		manifestData,
		0644,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// Craft file whitelist for zipping
	whitelist := map[string]bool{
		moscommon.GetManifestFilePath(""): true,
		localLibsDir:                      true,
		depsDir:                           true,
		".":                               true,
	}
	for _, v := range manifest.Sources {
		whitelist[ourfilepath.GetFirstPathComponent(v)] = true
	}
	for _, v := range manifest.Includes {
		whitelist[ourfilepath.GetFirstPathComponent(v)] = true
	}
	for _, v := range manifest.Filesystem {
		whitelist[ourfilepath.GetFirstPathComponent(v)] = true
	}
	for _, v := range manifest.BinaryLibs {
		whitelist[ourfilepath.GetFirstPathComponent(v)] = true
	}
	for _, v := range manifest.ExtraFiles {
		whitelist[ourfilepath.GetFirstPathComponent(v)] = true
	}

	transformers := make(map[string]fileTransformer)

	// create a zip out of the code dir
	os.Chdir(appStagingDir)
	src, err := zipUp(".", whitelist, transformers)
	if err != nil {
		return errors.Trace(err)
	}
	os.Chdir(appDir)

	// prepare multipart body
	body := &bytes.Buffer{}
	mpw := multipart.NewWriter(body)
	part, err := mpw.CreateFormFile(moscommon.FormSourcesZipName, "source.zip")
	if err != nil {
		return errors.Trace(err)
	}

	if _, err := part.Write(src); err != nil {
		return errors.Trace(err)
	}

	if *cleanBuildFlag {
		if err := mpw.WriteField(moscommon.FormCleanName, "1"); err != nil {
			return errors.Trace(err)
		}
	}

	pbValue := "0"
	if *preferPrebuiltLibs {
		pbValue = "1"
	}

	if err := mpw.WriteField(moscommon.FormPreferPrebuildLibsName, pbValue); err != nil {
		return errors.Trace(err)
	}

	bParamsYAML, err := yaml.Marshal(bParams)
	if err != nil {
		return errors.Trace(err)
	}
	if err := mpw.WriteField(moscommon.FormBuildParamsName, string(bParamsYAML)); err != nil {
		return errors.Trace(err)
	}

	if data, err := ioutil.ReadFile(moscommon.GetBuildCtxFilePath(buildDir)); err == nil {
		// Successfully read build context name, transmit it to the remote builder
		if err := mpw.WriteField(moscommon.FormBuildCtxName, string(data)); err != nil {
			return errors.Trace(err)
		}
	}

	if data, err := ioutil.ReadFile(moscommon.GetBuildStatFilePath(buildDir)); err == nil {
		// Successfully read build stat, transmit it to the remote builder
		if err := mpw.WriteField(moscommon.FormBuildStatName, string(data)); err != nil {
			return errors.Trace(err)
		}
	}

	if err := mpw.Close(); err != nil {
		return errors.Trace(err)
	}

	server, err := serverURL()
	if err != nil {
		return errors.Trace(err)
	}

	buildUser := "test"
	buildPass := "test"
	freportf(logWriterStderr, "Connecting to %s, user %s", server, buildUser)

	// invoke the fwbuild API (replace "master" with "latest")
	fwbuildVersion := version.GetMosVersion()

	if fwbuildVersion == "master" {
		fwbuildVersion = "latest"
	}

	uri := fmt.Sprintf("%s/api/fwbuild/%s/build", server, fwbuildVersion)

	freportf(logWriterStderr, "Uploading sources (%d bytes)", len(body.Bytes()))
	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", mpw.FormDataContentType())
	req.Header.Add("User-Agent", version.GetUserAgent())
	req.SetBasicAuth(buildUser, buildPass)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Trace(err)
	}

	// handle response
	body.Reset()
	body.ReadFrom(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusTeapot:
		// Build either succeeded or failed

		// unzip build results
		r := bytes.NewReader(body.Bytes())
		os.RemoveAll(buildDir)
		archive.UnzipInto(r, r.Size(), ".", 0)

		// Save local log
		ioutil.WriteFile(moscommon.GetBuildLogLocalFilePath(buildDir), logBuf.Bytes(), 0666)

		// print log in verbose mode or when build fails
		if *verbose || resp.StatusCode != http.StatusOK {
			log, err := os.Open(moscommon.GetBuildLogFilePath(buildDir))
			if err != nil {
				return errors.Trace(err)
			}
			io.Copy(os.Stdout, log)
		}

		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("build failed")
		}
		return nil

	default:
		// Unexpected response
		return errors.Errorf("error response: %d: %s", resp.StatusCode, strings.TrimSpace(body.String()))
	}
}

// copyExternalCode checks whether given path p is outside of appDir, and if
// so, copies its contents as a new directory under appStagingDir, and returns
// its name. If nothing was copied, returns an empty string.
func copyExternalCode(p, appDir, appStagingDir string) (string, error) {
	// Ensure we have relative path curPathRel which should start with ".." if
	// it's outside of the appStagingDir
	curPathAbs := p
	if !filepath.IsAbs(curPathAbs) {
		curPathAbs = filepath.Join(appDir, curPathAbs)
	}

	curPathAbs = filepath.Clean(curPathAbs)

	curPathRel, err := filepath.Rel(appDir, curPathAbs)
	if err != nil {
		return "", errors.Trace(err)
	}

	if len(curPathRel) > 0 && curPathRel[0] == '.' {
		// The path is outside of appStagingDir, so we should copy its contents
		// under appStagingDir

		// The path could end with a glob, so we need to get existing and
		// non-existing parts of the path
		//
		// TODO(dfrank): we should actually handle all the globs here in mos,
		// not in makefile.
		actualPart := curPathAbs
		globPart := ""

		if _, err := os.Stat(actualPart); err != nil {
			actualPart, globPart = filepath.Split(actualPart)
		}

		// Create a new directory named as a "blueprint" of the source directory:
		// full path with all separators replaced with "_".
		curTmpPathRel := strings.Replace(actualPart, string(filepath.Separator), "_", -1)

		curTmpPathAbs := filepath.Join(appStagingDir, curTmpPathRel)
		if err := os.MkdirAll(curTmpPathAbs, 0755); err != nil {
			return "", errors.Trace(err)
		}

		// Copy source files to that new dir
		// TODO(dfrank): ensure we don't copy too much
		freportf(logWriter, "Copying %q as %q", actualPart, curTmpPathAbs)
		err = ourio.CopyDir(actualPart, curTmpPathAbs, nil)
		if err != nil {
			return "", errors.Trace(err)
		}

		return filepath.Join(curTmpPathRel, globPart), nil
	}

	return "", nil
}

// copyExternalCodeAll calls copyExternalCode for each element of the paths
// slice, and for each affected path updates the item in the slice.
func copyExternalCodeAll(paths *[]string, appDir, appStagingDir string) error {
	for i, curPath := range *paths {
		newPath, err := copyExternalCode(curPath, appDir, appStagingDir)
		if err != nil {
			return errors.Trace(err)
		}

		if newPath != "" {
			(*paths)[i] = newPath
		}
	}

	return nil
}

// zipUp takes the whitelisted files and directories under path and returns an
// in-memory zip file. The whitelist map is applied to top-level dirs and files
// only. If some file needs to be transformed before placing into a zip
// archive, the appropriate transformer function should be placed at the
// transformers map.
func zipUp(
	dir string,
	whitelist map[string]bool,
	transformers map[string]fileTransformer,
) ([]byte, error) {
	data := &bytes.Buffer{}
	z := zip.NewWriter(data)

	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		// Zip files should always contain forward slashes
		fileForwardSlash := file
		if os.PathSeparator != rune('/') {
			fileForwardSlash = strings.Replace(file, string(os.PathSeparator), "/", -1)
		}
		parts := strings.Split(file, string(os.PathSeparator))

		if _, ok := whitelist[parts[0]]; !ok {
			glog.Infof("ignoring %q", file)
			if info.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}
		if info.IsDir() {
			return nil
		}

		if *verbose {
			ourutil.Reportf("Zipping %s", file)
		}

		w, err := z.Create(fileForwardSlash)
		if err != nil {
			return errors.Trace(err)
		}

		var r io.ReadCloser
		r, err = os.Open(file)
		if err != nil {
			return errors.Trace(err)
		}
		defer r.Close()

		t, ok := transformers[fileForwardSlash]
		if !ok {
			t = identityTransformer
		}

		r, err = t(r)
		if err != nil {
			return errors.Trace(err)
		}
		defer r.Close()

		if _, err := io.Copy(w, r); err != nil {
			return errors.Trace(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Trace(err)
	}

	z.Close()
	return data.Bytes(), nil
}

func identityTransformer(r io.ReadCloser) (io.ReadCloser, error) {
	return r, nil
}
