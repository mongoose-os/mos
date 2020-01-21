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
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"

	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/build"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/interpreter"
)

const (
	testManifestsDir   = "test_manifests"
	appDir             = "app"
	expectedDir        = "expected"
	finalManifestName  = "mos_final.yml"
	depsInitName       = "mgos_deps_init.c"
	testDescriptorName = "test_desc.yml"
	errorTextFile      = "error.txt"

	testPrefix    = "test_"
	testSetPrefix = "testset_"

	manifestParserRootPlaceholder = "__MANIFEST_PARSER_ROOT__"
	appRootPlaceholder            = "__APP_ROOT__"
	allPlatformsPlaceholder       = "__ALL_PLATFORMS__"
)

var (
	manifestParserRoot = ""
	repoRoot           = ""
)

type TestDescr struct {
	PreferBinaryLibs bool              `yaml:"prefer_binary_libs"`
	BuildVars        map[string]string `yaml:"build_vars"`
}

func init() {
	var err error
	manifestParserRoot, err = filepath.Abs(".")
	if err != nil {
		log.Fatal(err)
	}

	repoRoot, err = filepath.Abs("../..")
	if err != nil {
		log.Fatal(err)
	}
}

func TestParser(t *testing.T) {
	ok := handleTestSet(t, testManifestsDir)

	if !ok {
		t.Fatal("failing due the errors above")
	}
}

func handleTestSet(t *testing.T, testSetPath string) bool {
	files, err := ioutil.ReadDir(testSetPath)
	if err != nil {
		t.Fatal(errors.ErrorStack(err))
	}

	ok := true

	for _, f := range files {
		if strings.HasPrefix(f.Name(), testPrefix) {
			if err := singleManifestTest(t, filepath.Join(testSetPath, f.Name())); err != nil {
				t.Log(errors.ErrorStack(err))
				ok = false
			}
		} else if strings.HasPrefix(f.Name(), testSetPrefix) {
			if !handleTestSet(t, filepath.Join(testSetPath, f.Name())) {
				ok = false
			}
		}
	}

	return ok
}

func compareFiles(actualFilename, expectedFilename string) error {
	actualData, err := ioutil.ReadFile(actualFilename)
	if err != nil {
		return errors.Trace(err)
	}
	expectedData, err := ioutil.ReadFile(expectedFilename)
	if err != nil {
		return errors.Trace(err)
	}
	if bytes.Compare(expectedData, actualData) != 0 {
		return errors.Errorf("expected file %s doesn't match actual %s", expectedFilename, actualFilename)
	}
	return nil
}

func singleManifestTest(t *testing.T, appPath string) error {
	// Create test descriptor with default values
	descr := TestDescr{}

	// If test descriptor exists for the current test app, read it
	descrFilename := filepath.Join(appPath, testDescriptorName)
	if _, err := os.Stat(descrFilename); err == nil {
		descrData, err := ioutil.ReadFile(descrFilename)
		if err != nil {
			return errors.Trace(err)
		}

		if err := yaml.Unmarshal(descrData, &descr); err != nil {
			return errors.Trace(err)
		}
	}

	platformFiles, err := ioutil.ReadDir(filepath.Join(appPath, expectedDir))
	if err != nil {
		return errors.Trace(err)
	}

	platforms := []string{}

	for _, v := range platformFiles {
		platforms = append(platforms, v.Name())
	}

	for _, platform := range platforms {
		logWriter := &bytes.Buffer{}
		interp := interpreter.NewInterpreter(newMosVars())

		t.Logf("testing %q for %q %s descrFilename", appPath, platform, descrFilename)

		manifest, _, err := ReadManifestFinal(
			filepath.Join(appPath, appDir), &ManifestAdjustments{
				Platform:  platform,
				BuildVars: descr.BuildVars,
			}, logWriter, interp,
			&ReadManifestCallbacks{ComponentProvider: &compProviderTest{}}, true, descr.PreferBinaryLibs, 0,
		)

		expectedErrorFilename := filepath.Join(appPath, expectedDir, platform, errorTextFile)
		expectedErrorBytes, _ := ioutil.ReadFile(expectedErrorFilename)
		expectedError := strings.TrimSpace(string(expectedErrorBytes))

		if err != nil {
			if expectedError != "" {
				if strings.Contains(err.Error(), expectedError) {
					continue
				} else {
					return errors.Errorf("%s: expected error message to contain %q but it didn't (the message was: %q); see %s",
						appPath, expectedError, err.Error(), expectedErrorFilename)
				}
			}
			return errors.Trace(err)
		} else {
			if expectedError != "" {
				return errors.Errorf("%s: expected parsing to fail but it didn't", appPath)
			}
		}

		depsInitData, err := getDepsInitCCode(manifest)
		if err != nil {
			return errors.Trace(err)
		}

		data, err := yaml.Marshal(manifest)
		if err != nil {
			return errors.Trace(err)
		}

		data, err = addPlaceholders(data, appPath)
		if err != nil {
			return errors.Trace(err)
		}

		buildDir := moscommon.GetBuildDir(filepath.Join(appPath, appDir))
		os.MkdirAll(buildDir, 0777)

		actualFilename := filepath.Join(buildDir, finalManifestName)
		ioutil.WriteFile(actualFilename, data, 0644)
		expectedFilename := filepath.Join(appPath, expectedDir, platform, finalManifestName)

		if err = compareFiles(actualFilename, expectedFilename); err != nil {
			return errors.Trace(err)
		}

		actualFilename = filepath.Join(buildDir, depsInitName)
		ioutil.WriteFile(actualFilename, depsInitData, 0644)
		expectedFilename = filepath.Join(appPath, expectedDir, platform, depsInitName)
		if _, err := os.Stat(expectedFilename); err == nil {
			if err = compareFiles(actualFilename, expectedFilename); err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}

type compProviderTest struct{}

func (lpt *compProviderTest) GetLibLocalPath(
	m *build.SWModule, rootAppDir, libsDefVersion, platform string,
) (string, error) {
	appName, err := m.GetName()
	if err != nil {
		return "", errors.Trace(err)
	}

	return filepath.Join(rootAppDir, "..", "libs", appName), nil
}

func (lpt *compProviderTest) GetModuleLocalPath(
	m *build.SWModule, rootAppDir, modulesDefVersion, platform string,
) (string, error) {
	appName, err := m.GetName()
	if err != nil {
		return "", errors.Trace(err)
	}

	return filepath.Join(rootAppDir, "..", "modules", appName), nil
}

func (lpt *compProviderTest) GetMongooseOSLocalPath(
	rootAppDir, modulesDefVersion string,
) (string, error) {
	return repoRoot, nil
}

func newMosVars() *interpreter.MosVars {
	ret := interpreter.NewMosVars()
	ret.SetVar(interpreter.GetMVarNameMosVersion(), "0.01")
	return ret
}

func addPlaceholders(data []byte, appPath string) ([]byte, error) {
	data = []byte(strings.Replace(
		string(data),
		path.Join(manifestParserRoot, appPath),
		appRootPlaceholder,
		-1,
	))

	data = []byte(strings.Replace(
		string(data), manifestParserRoot, manifestParserRootPlaceholder, -1,
	))

	// All platforms placeholder
	allPlatforms, err := getAllSupportedPlatforms(repoRoot)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len(allPlatforms) == 0 {
		return nil, errors.Errorf("getAllSupportedPlatforms returned empty array")
	}

	allPlatformsStr := "- " + strings.Join(allPlatforms, "\n- ")

	data = []byte(strings.Replace(
		string(data), allPlatformsStr, allPlatformsPlaceholder, -1,
	))

	return data, nil
}
