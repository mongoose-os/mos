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
//go:generate go-bindata-assetfs -pkg manifest_parser -nocompress -mode 420 data/

// Check README.md for detailed explanation of parsing steps, limitations etc.

package manifest_parser

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	"github.com/juju/errors"
	flag "github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"
	glog "k8s.io/klog/v2"

	"github.com/mongoose-os/mos/cli/build"
	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/interpreter"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/version"
)

const (
	// Manifest version changes:
	//
	// - 2017-06-03: added support for @all_libs in filesystem and sources
	// - 2017-06-16: added support for conds with very basic expressions
	//               (only build_vars)
	// - 2017-09-29: added support for includes
	// - 2018-06-12: added support for globs in init_deps
	// - 2018-06-20: added no_implicit_init_deps
	// - 2018-08-13: added support for non-GitHub Git repos
	// - 2018-08-29: added support for adding libs under conds
	// - 2018-09-24: added special handling of the "boards" lib
	// - 2019-04-26: added warning and error
	// - 2019-07-28: added init_before
	// - 2020-01-21: added ability to override lib variants from conds in app manifest
	// - 2020-01-29: added ability to override app name, description and version from app's conds
	// - 2020-08-02: added asset_api for multiple asset-fetching mechanisms; fs_filters
	minManifestVersion = "2017-03-17"
	maxManifestVersion = "2020-08-02"

	depsApp = "app"

	allLibsKeyword = "@all_libs"

	assetPrefix           = "asset://"
	rootManifestAssetName = "data/root_manifest.yml"

	coreLibName     = "core"
	coreLibLocation = "https://github.com/mongoose-os-libs/core"

	supportedPlatforms = "cc3200 cc3220 esp32 esp8266 rs14100 stm32 ubuntu"
)

var (
	sourceGlobs = flag.StringSlice("source-glob", []string{"*.c", "*.cpp"}, "glob to use for source dirs. Can be used multiple times.")
)

type ComponentProvider interface {
	// GetLibLocalPath returns local path to the given software module.
	// NOTE that this method can be called concurrently for different modules.
	GetLibLocalPath(
		m *build.SWModule, rootAppDir, libsDefVersion, platform string,
	) (string, error)

	GetModuleLocalPath(
		m *build.SWModule, rootAppDir, modulesDefVersion, platform string,
	) (string, error)
}

type ReadManifestCallbacks struct {
	ComponentProvider ComponentProvider
}

type RMFOut struct {
	MTime time.Time

	MosDirEffective string

	AppSourceDirs []string
	AppFSDirs     []string
	AppBinLibDirs []string
}

type libPrepareResult struct {
	mtime time.Time
	err   error
}

func ReadManifestFinal(
	dir string, adjustments *build.ManifestAdjustments,
	logWriter io.Writer, interp *interpreter.MosInterpreter,
	cbs *ReadManifestCallbacks,
	requireArch, preferPrebuiltLibs bool,
	binaryLibsUpdateInterval time.Duration,
) (*build.FWAppManifest, *RMFOut, error) {
	interp = interp.Copy()

	if adjustments == nil {
		adjustments = &build.ManifestAdjustments{}
	}

	fp := &RMFOut{}
	buildDirAbs, err := filepath.Abs(moscommon.GetBuildDir(dir))
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest, mtime, err := readManifestWithLibs(
		dir, adjustments, logWriter, interp, cbs, requireArch,
	)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	if manifest.Name == "" {
		manifest.Name = filepath.Base(dir)
	}
	for _, c := range manifest.Name {
		if unicode.IsSpace(c) {
			return nil, nil, fmt.Errorf("app name (%q) should not contain spaces", manifest.Name)
		}
	}

	// Set the mos.platform variable
	interp.MVars.SetVar(interpreter.GetMVarNameMosPlatform(), manifest.Platform)

	if err := interpreter.SetManifestVars(interp.MVars, manifest); err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest.Name, err = interpreter.ExpandVars(interp, manifest.Name, false)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "while expanding name")
	}

	manifest.Author, err = interpreter.ExpandVars(interp, manifest.Author, false)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "while expanding author")
	}

	manifest.Version, err = interpreter.ExpandVars(interp, manifest.Version, false)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "while expanding version")
	}

	manifest.Summary, err = interpreter.ExpandVars(interp, manifest.Summary, false)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "while expanding summary")
	}

	manifest.Description, err = interpreter.ExpandVars(interp, manifest.Description, false)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "while expanding description")
	}

	var mosModule *build.SWModule
	for i, m := range manifest.Modules {
		if m.Name == build.MosModuleName {
			mosModule = &manifest.Modules[i]
			break
		}
	}
	if mosModule == nil {
		manifest.Modules = append(manifest.Modules, build.SWModule{
			Name:     build.MosModuleName,
			Location: build.MosDefaultRepo,
			Version:  manifest.MongooseOsVersion,
		})
	} else {
		if mosModule.Version == "" {
			mosModule.Version = manifest.MongooseOsVersion
		}
	}

	// Prepare local copies of all sw modules {{{
	// Modules are collected from the bottom of the dependency chain,
	// we go backwards to ensure overrides are handled first.
	modulesHandled := map[string]bool{}
	for i := len(manifest.Modules) - 1; i >= 0; i-- {
		m := &manifest.Modules[i]
		name, err := m.GetName()
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		if modulesHandled[name] {
			continue
		}

		moduleDir, err := cbs.ComponentProvider.GetModuleLocalPath(m, dir, manifest.ModulesVersion, manifest.Platform)
		if err != nil {
			return nil, nil, errors.Annotatef(err, "failed to prepare module %q", name)
		}

		interpreter.SetModuleVars(interp.MVars, name, moduleDir)
		modulesHandled[name] = true

		if name == build.MosModuleName {
			fp.MosDirEffective = moduleDir
		}
	}
	// }}}

	// Get sources and filesystem files from the manifest, expanding expressions {{{
	manifest.Sources, err = interpreter.ExpandVarsSlice(interp, manifest.Sources, false)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	for i, v := range manifest.LibsHandled {
		name, err := v.Lib.GetName()
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		interpreter.SetLibVars(interp.MVars, name, v.Path)
		manifest.LibsHandled[i].Sources, err = interpreter.ExpandVarsSlice(interp, v.Sources, false)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		manifest.LibsHandled[i].Version = v.Lib.GetVersion(manifest.LibsVersion)
		manifest.LibsHandled[i].UserVersion = v.Manifest.Version
		manifest.LibsHandled[i].RepoVersion, manifest.LibsHandled[i].RepoDirty, _ = v.Lib.GetRepoVersion()
	}

	manifest.Includes, err = interpreter.ExpandVarsSlice(interp, manifest.Includes, false)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	// AppSourceDirs will be populated later, needed to mount those paths to the
	// docker container
	fp.AppSourceDirs = []string{}

	manifest.Filesystem, err = interpreter.ExpandVarsSlice(interp, manifest.Filesystem, false)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest.BinaryLibs, err = interpreter.ExpandVarsSlice(interp, manifest.BinaryLibs, false)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest.Tests, err = interpreter.ExpandVarsSlice(interp, manifest.Tests, false)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest.Sources = prependPaths(manifest.Sources, dir)
	manifest.Includes = prependPaths(manifest.Includes, dir)
	manifest.Filesystem = prependPaths(manifest.Filesystem, dir)
	manifest.BinaryLibs = prependPaths(manifest.BinaryLibs, dir)

	manifest.Tests = prependPaths(manifest.Tests, dir)

	manifest.CFlags, err = interpreter.ExpandVarsSlice(interp, manifest.CFlags, false)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "while expanding cflags")
	}

	manifest.CXXFlags, err = interpreter.ExpandVarsSlice(interp, manifest.CXXFlags, false)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "while expanding cxxflags")
	}
	// }}}

	// Convert manifest.Sources into paths to concrete existing source files.
	manifest.Sources, fp.AppSourceDirs, err = resolvePaths(manifest.Sources, *sourceGlobs)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest.Filesystem, fp.AppFSDirs, err = resolvePaths(manifest.Filesystem, []string{"*"})
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	// Apply fs_filters.
	var newFs []string
	for i, f := range manifest.Filesystem {
		bf := filepath.Base(f)
		include := true
		for _, e := range manifest.FSFilters {
			if e.Include != "" && e.Exclude != "" {
				return nil, nil, errors.Errorf("fs_filters entry %d: only one of include or exclude is allowed", i)
			}
			if e.Include != "" {
				if matched, _ := path.Match(e.Include, bf); matched {
					include = true
					break
				}
			}
			if e.Exclude != "" {
				if matched, _ := path.Match(e.Exclude, bf); matched {
					glog.Infof("%q excluded by %q", f, e.Exclude)
					include = false
					break
				}
			}
		}
		if include {
			newFs = append(newFs, f)
		}
	}
	manifest.Filesystem = newFs
	manifest.FSFilters = nil

	// When building an app, also add all libs' sources or prebuilt binaries.
	if manifest.Type == build.ManifestTypeApp {
		for k, lcur := range manifest.LibsHandled {
			libSourceDirs := []string{}

			origSources := lcur.Sources
			// Convert dirs and globs to actual files
			manifest.LibsHandled[k].Sources, libSourceDirs, err = resolvePaths(lcur.Sources, *sourceGlobs)
			if err != nil {
				return nil, nil, errors.Trace(err)
			}

			// Check if binary version of the lib exists. We do this if there are
			// no sources or if we prefer binary libs (for speed).
			binaryLib := ""
			var fetchErrs []error
			if (len(manifest.LibsHandled[k].Sources) == 0 && len(origSources) != 0) || preferPrebuiltLibs {
				var variants []string
				if lcur.Lib.Variant != "" {
					variants = append(variants, lcur.Lib.Variant)
				}
				libVersion := lcur.Lib.GetVersion(manifest.LibsVersion)
				if v, ok := interp.MVars.GetVar("build_vars.BOARD"); ok && v.(string) != "" {
					variants = append(variants, fmt.Sprintf("%s-%s", manifest.Platform, v.(string)))
				}
				variants = append(variants, manifest.Platform)
				for _, variant := range variants {
					bl, err := filepath.Abs(moscommon.GetBinaryLibFilePath(buildDirAbs, lcur.Lib.Name, variant, libVersion))
					if err != nil {
						return nil, nil, errors.Trace(err)
					}
					fi, err := os.Stat(bl)
					if err == nil {
						// Local file exists, check it.
						// We want to re-fetch "latest" libs regularly (same way as repos get pulled).
						if libVersion != version.LatestVersionName || binaryLibsUpdateInterval == 0 ||
							fi.ModTime().Add(binaryLibsUpdateInterval).After(time.Now()) {
							if fi.Size() == 0 {
								// It's a tombstone, meaning this variant does not exist. Skip it.
								glog.V(1).Infof("%s is a tombstone, skipping", bl)
								continue
							}
							ourutil.Freportf(logWriter, "Prebuilt binary for %q already exists at %q", lcur.Lib.Name, bl)
							binaryLib = bl
							break
						}
					}
					// Try fetching
					fetchErr := lcur.Lib.FetchPrebuiltBinary(variant, libVersion, bl)
					if fetchErr == nil {
						ourutil.Freportf(logWriter, "Successfully fetched prebuilt binary for %q to %q", lcur.Lib.Name, bl)
						binaryLib = bl
					} else {
						fetchErrs = append(fetchErrs, fetchErr)
						if os.IsNotExist(errors.Cause(fetchErr)) {
							// This variant does not exist, create a tombstone to avoid fetching in the future.
							glog.V(1).Infof("%s: creating a tombstone", bl)
							ioutil.WriteFile(bl, nil, 0664)
						}
					}
					if binaryLib != "" {
						break
					}
				}
			}
			if binaryLib != "" {
				// We should use binary lib instead of sources
				manifest.LibsHandled[k].Sources = []string{}
				manifest.LibsHandled[k].BinaryLibs = append(manifest.LibsHandled[k].BinaryLibs, binaryLib)
				manifest.BinaryLibs = append(manifest.BinaryLibs, binaryLib)
			} else {
				// Use lib sources, not prebuilt binary
				if len(manifest.LibsHandled[k].Sources) == 0 && len(origSources) != 0 {
					// Originally the lib had some sources in its mos.yml, but turns out
					// that they don't exist (closed source lib), and we have failed to fetch a prebuilt
					// binary for it. Error out with a descriptive message.
					return nil, nil, errors.Errorf(
						"neither sources nor prebuilt binary exists for the lib %q "+
							"(or, if a library doesn't have any code by design, its mos.yml "+
							"shouldn't contain \"sources\"). Fetch error was: %s",
						manifest.LibsHandled[k].Lib.Name, fetchErrs,
					)
				}
				manifest.Sources = append(manifest.Sources, manifest.LibsHandled[k].Sources...)
			}

			fp.AppSourceDirs = append(fp.AppSourceDirs, libSourceDirs...)
		}

		// Generate deps manifest.
		dm, err := build.GenerateDepsManifest(manifest)
		if err != nil {
			return nil, nil, errors.Annotatef(err, "failed to generate deps manifest")
		}

		dmData, err := yaml.Marshal(dm)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}

		if err := os.MkdirAll(moscommon.GetGeneratedFilesDir(buildDirAbs), 0777); err != nil {
			return nil, nil, errors.Trace(err)
		}

		if err = ioutil.WriteFile(moscommon.GetDepsManifestFilePath(buildDirAbs), dmData, 0666); err != nil {
			return nil, nil, errors.Trace(err)
		}

		if adjustments.DepsVersions != nil {
			if err = build.ValidateDepsRequirements(dm, adjustments.DepsVersions); err != nil {
				if adjustments.StrictDepsVersions {
					return nil, nil, errors.Annotatef(err, "error validating dependencies (strict mode)")
				} else {
					ourutil.Freportf(logWriter, "%s", err)
					glog.Errorf("%s", err)
				}
			}
		}

		// Generate deps_init C code, and if it's not empty, write it to the temp
		// file and add to sources
		depsCCode, err := getDepsInitCCode(manifest, dm)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}

		if len(depsCCode) != 0 {
			fname := moscommon.GetDepsInitCFilePath(buildDirAbs)

			if err = ioutil.WriteFile(fname, depsCCode, 0666); err != nil {
				return nil, nil, errors.Trace(err)
			}

			// The modification time of autogenerated file should be set to that of
			// the manifest itself, so that make handles dependencies correctly.
			if err := os.Chtimes(fname, mtime, mtime); err != nil {
				return nil, nil, errors.Trace(err)
			}

			manifest.Sources = append(manifest.Sources, fname)
		}
	}

	manifest.BinaryLibs, fp.AppBinLibDirs, err = resolvePaths(manifest.BinaryLibs, []string{"*.a"})
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest.Tests, _, err = resolvePaths(manifest.Tests, []string{"*"})
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	allPlatforms, err := getAllSupportedPlatforms(fp.MosDirEffective)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	manifest.Platforms = mergeSupportedPlatforms(manifest.Platforms, allPlatforms)
	sort.Strings(manifest.Platforms)

	fp.MTime = mtime

	return manifest, fp, nil
}

var libsAddedError = errors.Errorf("new libs added")

type prepareLibsEntry struct {
	parentNodeName string
	manifest       *build.FWAppManifest
}

type manifestParseContext struct {
	// Directory of the "root" app. Might be a temporary directory.
	rootAppDir string

	adjustments build.ManifestAdjustments
	logWriter   io.Writer

	deps        *Deps
	initDeps    *Deps
	libsHandled map[string]*build.FWAppManifestLibHandled

	appManifest *build.FWAppManifest
	interp      *interpreter.MosInterpreter

	cbs *ReadManifestCallbacks

	requireArch bool

	prepareLibs []*prepareLibsEntry

	mtx        *sync.Mutex
	libsByName *libByNameMap
}

// readManifestWithLibs reads manifest from the provided dir, "expands" all
// libs (so that the returned manifest does not really contain any libs),
// and also returns the most recent modification time of all encountered
// manifests.
func readManifestWithLibs(
	dir string, adjustments *build.ManifestAdjustments,
	logWriter io.Writer, interp *interpreter.MosInterpreter,
	cbs *ReadManifestCallbacks,
	requireArch bool,
) (*build.FWAppManifest, time.Time, error) {
	interp = interp.Copy()
	libsHandled := map[string]*build.FWAppManifestLibHandled{}

	// Create a deps structure and add a root node: an "app"
	deps := NewDeps()
	deps.AddNode(depsApp)
	initDeps := NewDeps()
	initDeps.AddNode(depsApp)

	pc := &manifestParseContext{
		rootAppDir: dir,

		adjustments: *adjustments,
		logWriter:   logWriter,

		deps:        deps,
		initDeps:    initDeps,
		libsHandled: libsHandled,

		appManifest: nil,
		interp:      interp,

		requireArch: requireArch,

		cbs: cbs,

		mtx:        &sync.Mutex{},
		libsByName: newLibByNameMap(),
	}

	manifest, mtime, err := readManifestWithLibs2(dir, pc)
	if err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	pc.prepareLibs = append(pc.prepareLibs, &prepareLibsEntry{
		parentNodeName: depsApp,
		manifest:       manifest,
	})

	// Set the mos.platform variable
	interp.MVars.SetVar(interpreter.GetMVarNameMosPlatform(), manifest.Platform)

	pass := 0
	for {
		for len(pc.prepareLibs) != 0 {
			pass++
			glog.Infof("Prepare libs pass %d (%d)", pass, len(pc.prepareLibs))
			pll := pc.prepareLibs
			pc.prepareLibs = nil
			for _, ple := range pll {
				libsMtime, err := prepareLibs(ple.parentNodeName, ple.manifest, pc)
				if err != nil {
					return nil, time.Time{}, errors.Trace(err)
				} else {
					if libsMtime.After(mtime) {
						mtime = libsMtime
					}
				}
			}
		}

		// Get all deps in topological order
		depsTopo, cycle := deps.Topological(true)
		if cycle != nil {
			return nil, time.Time{}, errors.Errorf(
				"dependency cycle: %v", strings.Join(cycle, " -> "),
			)
		}

		// Remove the last item from topo, which is depsApp
		//
		// TODO(dfrank): it would be nice to handle an app just another dependency
		// and generate init code for it, but it would be a breaking change, at least
		// because all libs init functions return bool, but mgos_app_init returns
		// enum mgos_app_init_result.
		depsTopo = depsTopo[0 : len(depsTopo)-1]

		lhp := map[string]*build.FWAppManifestLibHandled{}
		for k, v := range libsHandled {
			vc := *v
			lhp[k] = &vc
		}

		// Expand initDeps (which may contain globs) against actual list of libs.
		initDepsExpanded := NewDeps()
		expandGlob := func(dep string, res *[]string) {
		deps:
			for _, d := range depsTopo {
				if m, _ := path.Match(dep, d); m {
					for _, rd := range *res {
						if d == rd {
							continue deps
						}
					}
					*res = append(*res, d)
				}
			}
			return
		}
		for _, node := range initDeps.GetNodes() {
			if node == depsApp {
				continue
			}
			// Expand globs in keys (intorduced by init_before)
			var nodeExpanded []string
			expandGlob(node, &nodeExpanded)
			if !(len(nodeExpanded) == 1 && nodeExpanded[0] == node) {
				glog.V(1).Infof("%s expanded to %s", node, nodeExpanded)
			}
			nodeDeps := initDeps.GetDeps(node)
			// Expand globs in values (introduced by init_after)
			var nodeDepsExpanded []string
			for _, nd := range nodeDeps {
				expandGlob(nd, &nodeDepsExpanded)
			}
			for _, ne := range nodeExpanded {
				initDepsExpanded.AddNodeWithDeps(ne, nodeDepsExpanded)
			}
		}
		for _, node := range initDepsExpanded.GetNodes() {
			nodeDepsExpanded := initDepsExpanded.GetDeps(node)
			sort.Strings(nodeDepsExpanded)
			glog.V(1).Infof("%s init deps: %s", node, nodeDepsExpanded)
			lhp[node].InitDeps = nodeDepsExpanded
		}

		initDepsTopo, cycle := initDepsExpanded.Topological(true)
		if cycle != nil {
			return nil, time.Time{}, errors.Errorf(
				"init dependency cycle: %v", strings.Join(cycle, " -> "),
			)
		}
		manifest.InitDeps = initDepsTopo

		// Create a LibsHandled slice in topological order computed above
		manifest.LibsHandled = make([]build.FWAppManifestLibHandled, 0, len(depsTopo))
		for _, v := range depsTopo {
			manifest.LibsHandled = append(manifest.LibsHandled, *lhp[v])
		}

		var lhNames []string
		for _, lh := range manifest.LibsHandled {
			lhn, _ := lh.Lib.GetName()
			lhNames = append(lhNames, lhn)
		}
		glog.Infof("libs_handled: %s", lhNames)
		glog.Infof("init_deps: %s", manifest.InitDeps)

		if err := expandManifestLibsAndConds(manifest, interp, adjustments); err != nil {
			if errors.Cause(err) == libsAddedError {
				if len(manifest.Libs) > 0 {
					libsMtime, err := prepareLibs(depsApp, manifest, pc)
					if err != nil {
						return nil, time.Time{}, errors.Trace(err)
					}
					if libsMtime.After(mtime) {
						mtime = libsMtime
					}
				}
				for _, lh := range manifest.LibsHandled {
					if len(lh.Manifest.Libs) > 0 {
						libsMtime, err := prepareLibs(lh.Lib.Name, lh.Manifest, pc)
						if err != nil {
							return nil, time.Time{}, errors.Trace(err)
						}
						if libsMtime.After(mtime) {
							mtime = libsMtime
						}
					}
				}
				continue
			}
			return nil, time.Time{}, errors.Trace(err)
		}

		if err := expandManifestAllLibsPaths(manifest); err != nil {
			return nil, time.Time{}, errors.Trace(err)
		}

		break
	}

	return manifest, mtime, nil
}

func readManifestWithLibs2(dir string, pc *manifestParseContext) (*build.FWAppManifest, time.Time, error) {
	manifest, mtime, err := ReadManifest(dir, &pc.adjustments, pc.interp)
	if err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	pc.mtx.Lock()
	defer pc.mtx.Unlock()

	if pc.requireArch && manifest.Platform == "" {
		return nil, time.Time{}, errors.Errorf("--platform must be specified or mos.yml should contain a platform key")
	}

	// If the given appManifest is nil, it means that we've just read one, so
	// remember it as such
	if pc.appManifest == nil {
		pc.appManifest = manifest

		if !manifest.NoImplInitDeps {
			found := false
			for _, l := range manifest.Libs {
				l.Normalize()
				if name, _ := l.GetName(); name == coreLibName {
					found = true
					break
				}
			}
			if !found {
				manifest.Libs = append(manifest.Libs, build.SWModule{
					Location: coreLibLocation,
				})
			}
		}

		for _, l := range pc.adjustments.ExtraLibs {
			l.Normalize()
			lName, _ := l.GetName()
			found := false
			for _, al := range manifest.Libs {
				al.Normalize()
				if name, _ := al.GetName(); name == lName {
					found = true
					break
				}
			}
			if !found {
				manifest.Libs = append(manifest.Libs, l)
			}
		}
		pc.adjustments.ExtraLibs = nil

		manifest.BuildVars["MGOS"] = "1"
		manifest.CDefs["MGOS"] = "1"

		for k, v := range pc.adjustments.CDefs {
			manifest.CDefs[k] = v
		}
		manifest.CFlags = append(manifest.CFlags, pc.adjustments.CFlags...)
		pc.adjustments.CFlags = nil
		manifest.CXXFlags = append(manifest.CXXFlags, pc.adjustments.CXXFlags...)
		pc.adjustments.CXXFlags = nil

		// Apply vars from the app manifest.
		// Since this we are at the top level, we can do it right now.
		interpreter.SetManifestVars(pc.interp.MVars, manifest)
	}

	return manifest, mtime, err
}

func prepareLibs(parentNodeName string, manifest *build.FWAppManifest, pc *manifestParseContext) (time.Time, error) {
	var wg sync.WaitGroup
	wg.Add(len(manifest.Libs))

	lpres := make(chan libPrepareResult, 1000)
	// Closer goroutine
	go func() {
		wg.Wait()
		close(lpres)
	}()

	for i := range manifest.Libs {
		go prepareLib(parentNodeName, &manifest.Libs[i], manifest, pc, lpres, &wg)
	}

	// Handle all lib prepare results
	var mtime time.Time
	for res := range lpres {
		if res.err != nil {
			return time.Time{}, errors.Trace(res.err)
		}

		// We should return the latest modification date of all encountered
		// manifests, so let's see if we got the later mtime here
		if res.mtime.After(mtime) {
			mtime = res.mtime
		}
	}

	manifest.Libs = nil

	return mtime, nil
}

// Met a library already handled while traversing the manifest tree.
//
// Two distinct scenarios are possible:
//  - A repeated library reference (typical and frequent):
//    -	Location is the same for libHad and libNow;
//    -	NB! libHad.Name is reliable, libNow.Name is NOT (isn't yet read from the
//  	library manifest).
//  - A library override (presumed relatively rare):
//    -	Location is different between libHad and libNow;
//    -	Name is the same for both.
//
// The library override scenario is currently somewhat fickle.  The location
// that will be used for a build is essentially the one processed earlier on.
// The processing order is only partially deterministic, however:
// - Distinct `passes' in readManifestWithLibs() split all unconditional library
//   references between sequential calls of prepareLibs(): first only those of
//   the top manifest, then those referenced additionally during the first pass
//   only, then those referenced additionally during the second pass only and so
//   on.  IOW, the library reference tree is handled top-down, level by level.
// - Within each such batch pass (i.e., each library tree level and each call to
//   prepareLibs()), however, calls of prepareLib() for each single library
//   reference are processed in parallel, and no strong ordering applies.
//   Therefore, to reliably override a library, one must do it from higher up
//   the manifest tree.
//
// Furthermore, library references from the top manifest file can override the
// variant setting of an already handled library.  Given the aforementioned pass
// ordering, that can't happen while handling unconditional library references.
// However, that can happen during the later condition expansion stage; as a
// matter of fact, the initial implementation of the variant overriding was
// targeting exactly that use case.  That obeys different ordering, see
// readManifestWithLibs() and commentary in expandManifestLibsAndConds().
func prepareLibReencounter(
	parentNodeName string, manifest *build.FWAppManifest,
	pc *manifestParseContext, libHad, libNow *build.SWModule,
) {
	if libHad.Location == libNow.Location {
		ourutil.Freportf(pc.logWriter, "Lib %q at %q: already handled...",
			libHad.Name, libHad.Location)
	} else {
		ourutil.Freportf(pc.logWriter, "Lib %q at %q: overridden by same at %q...",
			libHad.Name, libNow.Location, libHad.Location)
	}

	// Note the dependency.
	pc.mtx.Lock()
	pc.deps.AddDep(parentNodeName, libHad.Name)
	if !manifest.NoImplInitDeps {
		pc.initDeps.AddDep(parentNodeName, libHad.Name)
	}
	pc.mtx.Unlock()

	// App manifest can override library variants (in conds).
	if libNow.Variant != "" && parentNodeName == depsApp {
		glog.V(1).Infof("%s variant: %q -> %q", libHad.Name,
			libHad.Variant, libNow.Variant)
		libHad.Variant = libNow.Variant
	}
}

func prepareLib(
	parentNodeName string, m *build.SWModule,
	manifest *build.FWAppManifest,
	pc *manifestParseContext,
	lpres chan libPrepareResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	// Stash the name explicitly set in the referring manifest, if any.
	// m.Name can change before its original value may need to be examined.
	libRefName := m.Name
	if err := m.Normalize(); err != nil {
		lpres <- libPrepareResult{err: errors.Trace(err)}
		return
	}

	ls := pc.libsByName.AddOrFetchAndLock(m.Name)
	defer ls.mtx.Unlock()
	if ls.Lib != nil {
		prepareLibReencounter(parentNodeName, manifest, pc, ls.Lib, m)
		return
	}

	ourutil.Freportf(pc.logWriter, "Reading lib %q at %q...", m.Name, m.Location)

	libLocalDir, err := pc.cbs.ComponentProvider.GetLibLocalPath(
		m, pc.rootAppDir, pc.appManifest.LibsVersion, manifest.Platform,
	)
	if err != nil {
		lpres <- libPrepareResult{err: errors.Trace(err)}
		return
	}

	libLocalDir, err = filepath.Abs(libLocalDir)
	if err != nil {
		lpres <- libPrepareResult{err: errors.Trace(err)}
		return
	}

	pc.mtx.Lock()
	// If platform is empty, we need to set it from the outer manifest,
	// because arch is used in libs to handle arch-dependent submanifests, like
	// mos_esp8266.yml.
	if pc.adjustments.Platform == "" {
		pc.adjustments.Platform = manifest.Platform
	}
	pc.mtx.Unlock()

	libManifest, libMtime, err := readManifestWithLibs2(libLocalDir, pc)
	if err != nil {
		lpres <- libPrepareResult{err: errors.Trace(err)}
		return
	}

	// The name of a library can be explicitly set in its manifest and/or in the
	// referring manifest.  Barring that, the name defaults to the location
	// basename.
	//
	// The library code is hardwired to the right name via at least its
	// mgos_*_init() symbol.  The code using the library may also be via, e.g.,
	// HAVE_* variables.
	//
	// The building process uses the library name for library deduplication and
	// overriding.
	//
	// The location being `.../foo', the library name in use is:
	//
	// (#)  lib mos.yml  ref mos.yml  name in use
	// --------------------------------------
	// (1)  (none)       (none)       foo
	// (2)  (none)       foo          foo
	// (3)  (none)       bar          bar
	// (4)  foo          (none)       foo
	// (5)  foo          foo          foo
	// (6)  foo          bar          (ERROR)
	// (7)  bar          (none)       (ERROR)
	// (8)  bar          foo          (ERROR)
	// (9)  bar          bar          bar
	//
	// Rationales per (#) case:
	// - (1) The most typical use case.
	// - (2, 4, 5) Effectively no new information atop the location basename.
	// - (3) The handy use case of, e.g., .../foo-test1, .../foo-test2 copies
	//   adjacent to one another and/or the original .../foo.
	// - (6) If `foo' were correct here, the same problem as in (7) would apply.
	//   If `bar' were correct, then `foo' in lib mos.yml would produce erroneous
	//   results via any automation/content generation applied to individual
	//   libraries outside their usages.
	// - (7) The name set via the library manifest must be replicated in the
	//   referring manifest.  Otherwise, if this library is overridden while this
	//   particular location isn't accessible from the building environment,
	//   library deduplication/overriding by name can't obviate the need to read
	//   the inaccessible manifest.
	// - (8) This is (6) with the names swapped around.
	// - (9) A location-independently named library.  E.g., Github repositories
	//   .../mgos-foo, .../mgos-bar with libraries next to repositories unrelated
	//   to Mongoose OS.
	//
	// At this point, libRefName == `ref mos.yml' name above; libManifest.name ==
	// `lib mos.yml' name; m.Name == libRefName || library location basename.
	// After validation, m.Name will be the library `name to use' above.
	if libManifest.Name != "" {
		if libRefName != "" && libRefName != libManifest.Name { // (6, 8) above
			lpres <- libPrepareResult{
				err: fmt.Errorf("Library %q at %q is referred to as %q from %q",
					libManifest.Name, m.Location,
					libRefName, manifest.Origin),
			}
			return
		}
		if libRefName == "" && m.Name != libManifest.Name { // (7) above
			lpres <- libPrepareResult{
				err: fmt.Errorf("Library %q at %q must be referred to as %q from %q",
					libManifest.Name, m.Location,
					libManifest.Name, manifest.Origin),
			}
			return
		}
	}
	if libRefName != "" && m.Name != libRefName { // (3, 9) above
		m.Name = libRefName
	}
	name, err := m.GetName()
	if err != nil {
		lpres <- libPrepareResult{err: errors.Trace(err)}
		return
	}

	ourutil.Freportf(pc.logWriter, "Handling lib %q at %q...", name, m.Location)

	// Prep a build var and C macro MGOS_HAVE_<lib_name>
	haveName := fmt.Sprintf(
		"MGOS_HAVE_%s", strings.ToUpper(ourutil.IdentifierFromString(name)),
	)

	pc.mtx.Lock()
	lh := pc.libsHandled[name]
	if lh != nil {
		pc.mtx.Unlock()
		ls.Lib = &lh.Lib
		prepareLibReencounter(parentNodeName, manifest, pc,
			&pc.libsHandled[name].Lib, m)
		return
	}

	pc.prepareLibs = append(pc.prepareLibs, &prepareLibsEntry{
		parentNodeName: name,
		manifest:       libManifest,
	})

	// Now that we know we need to handle current lib, add a node for it
	pc.deps.AddNode(name)
	pc.initDeps.AddNode(name)
	pc.deps.AddDep(parentNodeName, name)
	if !manifest.NoImplInitDeps {
		pc.initDeps.AddDep(parentNodeName, name)
	}

	manifest.BuildVars[haveName] = "1"
	manifest.CDefs[haveName] = "1"

	lh = &build.FWAppManifestLibHandled{
		Lib:      *m,
		Path:     libLocalDir,
		Deps:     pc.deps.GetDeps(name),
		Manifest: libManifest,
	}
	pc.libsHandled[name] = lh
	ls.Lib = &lh.Lib
	pc.initDeps.AddNodeWithDeps(name, libManifest.InitAfter)
	if !libManifest.NoImplInitDeps && name != coreLibName {
		// Implicit dep on "core"
		pc.initDeps.AddDep(name, coreLibName)
	}
	for _, dep := range libManifest.InitBefore {
		pc.initDeps.AddNodeWithDeps(dep, []string{name})
	}
	pc.mtx.Unlock()

	lpres <- libPrepareResult{mtime: libMtime}
}

// ReadManifest reads manifest file(s) from the specific directory; if the
// manifest or given BuildParams have arch specified, then the returned
// manifest will contain all arch-specific adjustments (if any)
func ReadManifest(
	appDir string, adjustments *build.ManifestAdjustments, interp *interpreter.MosInterpreter,
) (*build.FWAppManifest, time.Time, error) {
	interp = interp.Copy()

	if adjustments == nil {
		adjustments = &build.ManifestAdjustments{}
	}

	manifestFullName := moscommon.GetManifestFilePath(appDir)
	manifest, mtime, err := ReadManifestFile(manifestFullName, interp, true)
	if err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	// Override arch with the value given in command line
	if adjustments.Platform != "" {
		manifest.Platform = adjustments.Platform
	}
	manifest.Platform = strings.ToLower(manifest.Platform)

	// Set the mos.platform variable
	interp.MVars.SetVar(interpreter.GetMVarNameMosPlatform(), manifest.Platform)

	// If type is omitted, assume "app"
	if manifest.Type == "" {
		manifest.Type = build.ManifestTypeApp
	}

	if manifest.Platform != "" {
		manifestArchFullName := moscommon.GetManifestArchFilePath(appDir, manifest.Platform)
		_, err := os.Stat(manifestArchFullName)
		if err == nil {
			// Arch-specific mos.yml does exist, so, handle it
			archManifest, archMtime, err := ReadManifestFile(manifestArchFullName, interp, false)
			if err != nil {
				return nil, time.Time{}, errors.Trace(err)
			}

			// We should return the latest modification date of all encountered
			// manifests, so let's see if we got the later mtime here
			if archMtime.After(mtime) {
				mtime = archMtime
			}

			// Extend common app manifest with arch-specific things.
			if err := extendManifest(manifest, manifest, archManifest, "", "", interp, &extendManifestOptions{
				skipFailedExpansions: true,
				extendInitDeps:       true,
			}); err != nil {
				return nil, time.Time{}, errors.Trace(err)
			}
		} else if !os.IsNotExist(err) {
			// Some error other than non-existing mos_<arch>.yml; complain.
			return nil, time.Time{}, errors.Trace(err)
		}
	}

	if manifest.Platforms == nil {
		manifest.Platforms = []string{}
	}

	// Apply adjustments (other than Platform which was applied earlier)
	if err := extendManifest(
		manifest, manifest, &build.FWAppManifest{
			BuildVars: adjustments.BuildVars,
		}, "", "", interp, &extendManifestOptions{
			skipFailedExpansions: true,
		},
	); err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	return manifest, mtime, nil
}

func checkWarningAndError(manifest *build.FWAppManifest) error {
	if manifest.Error != "" {
		ourutil.Reportf("Error: %s: %s", manifest.Origin, manifest.Error)
		return errors.Errorf("%s: %s", manifest.Origin, manifest.Error)
	}
	if manifest.Warning != "" {
		ourutil.Reportf("Warning: %s: %s", manifest.Origin, manifest.Warning)
	}
	return nil
}

// ReadManifestFile reads single manifest file (which can be either "main" app
// or lib manifest, or some arch-specific adjustment manifest)
func ReadManifestFile(
	manifestFullName string, interp *interpreter.MosInterpreter, manifestVersionMandatory bool,
) (*build.FWAppManifest, time.Time, error) {
	interp = interp.Copy()
	var manifestSrc []byte
	var err error

	if !strings.HasPrefix(manifestFullName, assetPrefix) {
		// Reading regular file from the host filesystem
		manifestSrc, err = ioutil.ReadFile(manifestFullName)
	} else {
		// Reading the asset
		assetName := manifestFullName[len(assetPrefix):]
		manifestSrc, err = Asset(assetName)
	}
	if err != nil {
		return nil, time.Time{}, errors.Annotatef(err, "reading manifest %q", manifestFullName)
	}

	var manifest build.FWAppManifest
	if err := yaml.Unmarshal(manifestSrc, &manifest); err != nil {
		return nil, time.Time{}, errors.Annotatef(err, "parsing manifest %q", manifestFullName)
	}

	manifest.Origin = manifestFullName

	if manifest.ManifestVersion != "" {
		// Check if manifest manifest version is supported by the mos tool
		if manifest.ManifestVersion < minManifestVersion {
			return nil, time.Time{}, errors.Errorf(
				"too old manifest_version %q in %q (oldest supported is %q)",
				manifest.ManifestVersion, manifestFullName, minManifestVersion,
			)
		}

		if manifest.ManifestVersion > maxManifestVersion {
			return nil, time.Time{}, errors.Errorf(
				"too new manifest_version %q in %q (latest supported is %q). Please run \"mos update\".",
				manifest.ManifestVersion, manifestFullName, maxManifestVersion,
			)
		}
	} else if manifestVersionMandatory {
		return nil, time.Time{}, errors.Errorf(
			"manifest version is missing in %q", manifestFullName,
		)
	}

	if err = checkWarningAndError(&manifest); err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	for i, _ := range manifest.Modules {
		if err = manifest.Modules[i].Normalize(); err != nil {
			return nil, time.Time{}, errors.Trace(err)
		}
	}

	if manifest.BuildVars == nil {
		manifest.BuildVars = make(map[string]string)
	}

	if manifest.CDefs == nil {
		manifest.CDefs = make(map[string]string)
	}

	if manifest.MongooseOsVersion == "" {
		manifest.MongooseOsVersion = interpreter.WrapMosExpr(interpreter.GetMVarNameMosVersion())
	}

	if manifest.LibsVersion == "" {
		manifest.LibsVersion = interpreter.WrapMosExpr(interpreter.GetMVarNameMosVersion())
	}

	if manifest.ModulesVersion == "" {
		manifest.ModulesVersion = interpreter.WrapMosExpr(interpreter.GetMVarNameMosVersion())
	}

	if manifest.Platform == "" && manifest.ArchOld != "" {
		manifest.Platform = manifest.ArchOld
	}

	manifest.MongooseOsVersion, err = interpreter.ExpandVars(interp, manifest.MongooseOsVersion, false)
	if err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	manifest.LibsVersion, err = interpreter.ExpandVars(interp, manifest.LibsVersion, false)
	if err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	manifest.ModulesVersion, err = interpreter.ExpandVars(interp, manifest.ModulesVersion, false)
	if err != nil {
		return nil, time.Time{}, errors.Trace(err)
	}

	var modTime time.Time

	if !strings.HasPrefix(manifestFullName, assetPrefix) {
		stat, err := os.Stat(manifestFullName)
		if err != nil {
			return nil, time.Time{}, errors.Trace(err)
		}

		modTime = stat.ModTime()
	}

	return &manifest, modTime, nil
}

// expandManifestLibsAndConds takes a manifest and expands all LibsHandled
// and Conds inside all manifests (app and all libs). Since expanded
// conds should be applied in topological order, the process is a bit
// involved:
//
// 1. Create copy of the app manifest: commonManifest
// 2. Expand all libs into that commonManifest
// 3. If resulting manifest has no conds, we're done. Otherwise:
//   a. For each of the manifests (app and all libs), expand conds, but
//      evaluate cond expressions against the commonManifest
//   b. Go to step 1
func expandManifestLibsAndConds(
	manifest *build.FWAppManifest, interp *interpreter.MosInterpreter,
	adjustments *build.ManifestAdjustments,
) error {
	interp = interp.Copy()

	// First of all, read root manifest since it should be the first manifest
	// in the chain (see below)
	rootManifest, _, err := ReadManifestFile(
		fmt.Sprint(assetPrefix, rootManifestAssetName), interp, true,
	)
	if err != nil {
		return errors.Trace(err)
	}

	// We need everything under root manifest's conds to be already available, so
	// expand all conds there. It means that the conds in root manifest should
	// only depend on the stuff already defined (basically, only "mos.platform").
	//
	// TODO(dfrank): probably make it so that if conds expression fails to
	// evaluate, keep it unexpanded for now.
	if err := ExpandManifestConds(rootManifest, rootManifest, interp, false); err != nil {
		return errors.Trace(err)
	}

	for {
		// First, we build a chain of all manifests we have:
		//
		// - Dummy empty manifest (needed so that extendManifest() will be called
		//   with the actual first manifest as "m2", and thus will expand
		//   expressions in its BuildVars and CDefs)
		// - Root manifest
		// - All libs (if any), starting from the one without any deps
		// - App
		allManifests := []*build.FWAppManifestLibHandled{}
		allManifests = append(allManifests, &build.FWAppManifestLibHandled{
			Lib:      build.SWModule{Name: "dummy_empty_manifest"},
			Path:     "",
			Manifest: &build.FWAppManifest{},
		})

		allManifests = append(allManifests, &build.FWAppManifestLibHandled{
			Lib:      build.SWModule{Name: "root_manifest"},
			Path:     "",
			Manifest: rootManifest,
		})

		for k, _ := range manifest.LibsHandled {
			allManifests = append(allManifests, &manifest.LibsHandled[k])
		}
		allManifests = append(allManifests, &build.FWAppManifestLibHandled{
			Lib:      build.SWModule{Name: "app"},
			Path:     "",
			Manifest: manifest,
		})

		// Set commonManifest to the first manifest in the deps chain, which is
		// a dummy empty manifest.
		commonManifest := allManifests[0].Manifest

		// Iterate all the rest of the manifests, at every step extending the
		// current one with all previous manifests accumulated so far, and the
		// current one takes precedence.
		for k := 1; k < len(allManifests); k++ {
			lcur := allManifests[k]

			curManifest := *lcur.Manifest

			lcur.Sources = prependPaths(curManifest.Sources, lcur.Path)

			if err := extendManifest(
				&curManifest, commonManifest, &curManifest, "", lcur.Path, interp, &extendManifestOptions{
					skipSources: true,
				},
			); err != nil {
				return errors.Annotatef(err, "expanding %q", lcur.Lib.Name)
			}

			commonManifest = &curManifest
		}

		// Now, commonManifest has everything expanded. Let's see if it contains
		// non-expanded conds.

		if len(commonManifest.Conds) == 0 {
			// No more conds in the common manifest, so cleanup all libs manifests,
			// and return commonManifest

			for _, l := range commonManifest.LibsHandled {
				l.Manifest = nil
			}
			*manifest = *commonManifest

			return nil
		}

		// There are some conds to be expanded. We can't expand them directly in
		// the common manifest, because items should be inserted in topological
		// order. Instead, we'll expand conds separately in the source app
		// manifest, and in each lib's manifests, but we'll execute the cond
		// expressions against the common manifest which we've just computed above,
		// so it already has everything properly overridden.
		//
		// When it's done, we'll expand all libs manifests again, etc, until there
		// are no conds left.

		// Note(rojer): Order of evaluation here is a bit strange:
		// top-level (app) conds are evaluated first, and then evaluation proceeds
		// from the bottom (starting with libs with no deps).

		if err := ExpandManifestConds(manifest, commonManifest, interp, true); err != nil {
			return errors.Annotatef(err, "expanding app manifest's conds")
		}
		if len(manifest.Libs) > 0 {
			glog.V(2).Infof("New libs added while expanding app conds, restarting eval...")
			return libsAddedError
		}

		for _, l := range manifest.LibsHandled {
			if l.Manifest != nil && len(l.Manifest.Conds) > 0 {
				if err := ExpandManifestConds(l.Manifest, commonManifest, interp, false); err != nil {
					return errors.Annotatef(err, "expanding %q conds", l.Lib.Name)
				}
				if len(l.Manifest.Libs) > 0 {
					glog.V(2).Infof("New libs added while expnading conds for %s, restarting eval...", l.Lib.Name)
					return libsAddedError
				}
			}
		}
	}
}

// expandManifestAllLibsPaths expands "@all_libs" for manifest's Sources
// and Filesystem paths
func expandManifestAllLibsPaths(manifest *build.FWAppManifest) error {
	var err error

	manifest.Sources, err = expandAllLibsPaths(manifest.Sources, manifest.LibsHandled)
	if err != nil {
		return errors.Trace(err)
	}

	manifest.Includes, err = expandAllLibsPaths(manifest.Includes, manifest.LibsHandled)
	if err != nil {
		return errors.Trace(err)
	}

	manifest.Filesystem, err = expandAllLibsPaths(manifest.Filesystem, manifest.LibsHandled)
	if err != nil {
		return errors.Trace(err)
	}

	manifest.BinaryLibs, err = expandAllLibsPaths(manifest.BinaryLibs, manifest.LibsHandled)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// expandAllLibsPaths expands "@all_libs" for the given paths slice, and
// returns a new slice
func expandAllLibsPaths(
	paths []string, libsHandled []build.FWAppManifestLibHandled,
) ([]string, error) {
	ret := []string{}

	for _, p := range paths {
		if strings.HasPrefix(p, allLibsKeyword) {
			innerPath := p[len(allLibsKeyword):]
			for _, lh := range libsHandled {
				ret = append(ret, filepath.Join(lh.Path, innerPath))
			}
		} else {
			ret = append(ret, p)
		}
	}

	return ret, nil
}

// ExpandManifestConds expands all "conds" in the dstManifest, but all cond
// "when" expressions are evaluated against the refManifest. Nested conds are
// not expanded: if there are some new conds left, a new refManifest should be
// computed by the caller, and ExpandManifestConds should be called again for
// each lib's manifest and for app's manifest.
//
// NOTE that although cond "when" expressions are evaluated against refManifest,
// expressions inside of the conditionally-applied manifest (like
// `${build_vars.FOO} bar`) are expanded against dstManifest. See README.md,
// Step 3 for details.
func ExpandManifestConds(
	dstManifest, refManifest *build.FWAppManifest, interp *interpreter.MosInterpreter, isAppManifest bool,
) error {
	interp = interp.Copy()

	// As we're expanding conds, we need to remove the conds themselves. But
	// extending manifest could cause new conds to be added, so we just save
	// current conds from the manifest in a separate variable, and clean the
	// manifest's conds. This way, newly added conds (if any) won't mess
	// with the old ones.
	conds := dstManifest.Conds
	dstManifest.Conds = nil

	if err := interpreter.SetManifestVars(interp.MVars, refManifest); err != nil {
		return errors.Trace(err)
	}

	for i, cond := range conds {
		res, err := interp.EvaluateExprBool(cond.When)
		if err != nil {
			return errors.Annotatef(err, "evaluating cond %q expression '%s'", "when", cond.When)
		}

		if !res {
			// The condition is false, skip handling
			continue
		}

		// If error is not an empty string, it means misconfiguration of
		// the current app, so, return an error
		if cond.Error != "" {
			return errors.New(cond.Error)
		}

		// Apply submanifest if present
		if cond.Apply != nil {
			cond.Apply.Origin = fmt.Sprintf("%s cond %d", dstManifest.Origin, i+1)
			if err := extendManifest(dstManifest, dstManifest, cond.Apply, "", "", interp, &extendManifestOptions{
				skipFailedExpansions: true,
			}); err != nil {
				return errors.Trace(err)
			}
			if isAppManifest && cond.Apply.Name != "" {
				dstManifest.Name = cond.Apply.Name
			}
			if cond.Apply.Description != "" {
				dstManifest.Description = cond.Apply.Description
			}
			if cond.Apply.Version != "" {
				dstManifest.Version = cond.Apply.Version
			}
		}
	}

	return nil
}

// extendManifest extends one manifest with another one.
//
// Currently there are two use cases for it:
// - when extending app's manifest with library's manifest;
// - when extending common app's manifest with the arch-specific one.
//
// These cases have different semantics: in the first case, the app's manifest
// should take precedence, but in the second case, the arch-specific manifest
// should take the precedence over that of an app. But NOTE: in both cases,
// it's app's manifest which should get extended.
//
// So, extendManifest takes 3 pointers to manifest:
// - mMain: main manifest which will be extended;
// - m1: lower-precedence manifest (which goes "first", this matters e.g.
//   for config_schema);
// - m2: higher-precedence manifest (which goes "second").
//
// mMain should typically be the same as either m1 or m2.
//
// m2 takes precedence over m1, and can depend on things defined in m1. So
// e.g. when extending app manifest with lib manifest, lib should be m1, app
// should be m2: config schema defined in lib will go before that of an app,
// and if both an app and a lib define the same build variable, app will win.
//
// m1Dir and m2Dir are optional paths for manifests m1 and m2, respectively.
// If the dir is not empty, then it gets prepended to each source and
// filesystem entry (except entries with absolute paths or paths starting with
// a variable)
func extendManifest(
	mMain, m1, m2 *build.FWAppManifest, m1Dir, m2Dir string,
	interp *interpreter.MosInterpreter, opts *extendManifestOptions,
) error {
	interp = interp.Copy()

	if opts == nil {
		opts = &extendManifestOptions{}
	}

	if err := checkWarningAndError(m1); err != nil {
		return errors.Trace(err)
	}

	if err := checkWarningAndError(m2); err != nil {
		return errors.Trace(err)
	}

	// Extend sources
	if !opts.skipSources {
		mMain.Sources = append(
			prependPaths(m1.Sources, m1Dir),
			prependPaths(m2.Sources, m2Dir)...,
		)
	}

	// Extend include paths
	mMain.Includes = append(
		prependPaths(m1.Includes, m1Dir),
		prependPaths(m2.Includes, m2Dir)...,
	)
	// Extend filesystem
	mMain.Filesystem = append(
		prependPaths(m1.Filesystem, m1Dir),
		prependPaths(m2.Filesystem, m2Dir)...,
	)
	// Extend binary libs
	mMain.BinaryLibs = append(
		prependPaths(m1.BinaryLibs, m1Dir),
		prependPaths(m2.BinaryLibs, m2Dir)...,
	)

	// Add modules from the lib, dedup and override if necessary.
	mm := make(map[string]build.SWModule)
	for _, m := range append(m1.Modules, m2.Modules...) {
		// Module must already be normalized at this point.
		if m.Name == "" {
			return fmt.Errorf("module not normalized! %+v", m)
		}
		mm[m.Name] = m
	}
	mMain.Modules = nil
	for _, m := range mm {
		mMain.Modules = append(mMain.Modules, m)
	}
	sort.Slice(mMain.Modules, func(i, j int) bool { return mMain.Modules[i].Name < mMain.Modules[j].Name })

	// Add modules libs from the lib.
	mMain.Libs = append(m1.Libs, m2.Libs...)
	mMain.ConfigSchema = append(m1.ConfigSchema, m2.ConfigSchema...)
	mMain.CFlags = append(m1.CFlags, m2.CFlags...)
	mMain.CXXFlags = append(m1.CXXFlags, m2.CXXFlags...)
	if opts.extendInitDeps {
		mMain.InitAfter = append(m1.InitAfter, m2.InitAfter...)
		mMain.InitBefore = append(m1.InitBefore, m2.InitBefore...)
	}

	// m2.BuildVars and m2.CDefs can contain expressions which should be expanded
	// against manifest m1.
	if err := interpreter.SetManifestVars(interp.MVars, m1); err != nil {
		return errors.Trace(err)
	}

	var err error

	mMain.BuildVars, err = mergeMapsString(m1.BuildVars, m2.BuildVars, interp, opts.skipFailedExpansions)
	if err != nil {
		return errors.Annotatef(err, "handling build_vars")
	}

	mMain.CDefs, err = mergeMapsString(m1.CDefs, m2.CDefs, interp, opts.skipFailedExpansions)
	if err != nil {
		return errors.Annotatef(err, "handling cdefs")
	}

	mMain.Platforms = mergeSupportedPlatforms(m1.Platforms, m2.Platforms)

	// Extend conds
	mMain.Conds = append(
		prependCondPaths(m1.Conds, m1Dir),
		prependCondPaths(m2.Conds, m2Dir)...,
	)

	return nil
}

type extendManifestOptions struct {
	skipSources          bool
	skipFailedExpansions bool
	extendInitDeps       bool
}

func prependPaths(items []string, dir string) []string {
	ret := []string{}
	for _, s := range items {
		prefix := ""
		if s[0] == '-' || s[0] == '+' {
			prefix = fmt.Sprintf("%c", s[0])
			s = s[1:]
		}

		// If the path is not absolute, and does not start with the variable,
		// prepend it with the library's path
		if dir != "" && s[0] != '$' && s[0] != '@' && !filepath.IsAbs(s) {
			s = filepath.Join(dir, s)
		}
		ret = append(ret, prefix+s)
	}
	return ret
}

// prependCondPaths takes a slice of "conds", and for each of them which
// contains an "apply" clause (effectively, a submanifest), prepends paths of
// sources and filesystem with the given dir.
func prependCondPaths(conds []build.ManifestCond, dir string) []build.ManifestCond {
	ret := []build.ManifestCond{}
	for _, c := range conds {
		if c.Apply != nil {
			subManifest := *c.Apply
			subManifest.Sources = prependPaths(subManifest.Sources, dir)
			subManifest.Includes = prependPaths(subManifest.Includes, dir)
			subManifest.Filesystem = prependPaths(subManifest.Filesystem, dir)
			subManifest.BinaryLibs = prependPaths(subManifest.BinaryLibs, dir)
			c.Apply = &subManifest
		}
		ret = append(ret, c)
	}
	return ret
}

// mergeMapsString merges two map[string]string into a new one; m2 takes
// precedence over m1. Values of m2 can contain expressions which are expanded
// against the given interp.
func mergeMapsString(
	m1, m2 map[string]string, interp *interpreter.MosInterpreter, skipFailed bool,
) (map[string]string, error) {
	bv := make(map[string]string)

	for k, v := range m1 {
		bv[k] = v
	}
	for k, v := range m2 {
		var err error
		bv[k], err = interpreter.ExpandVars(interp, v, skipFailed)
		if err != nil {
			return nil, errors.Annotatef(err, "handling %q", k)
		}
	}

	return bv, nil
}

// mergeSupportedPlatforms returns a slice of all strings which are contained
// in both p1 and p2, or if one of slices is empty, returns another one.
func mergeSupportedPlatforms(p1, p2 []string) []string {
	if len(p1) == 0 {
		return p2
	} else if len(p2) == 0 {
		return p1
	} else {
		m := map[string]struct{}{}
		for _, v := range p1 {
			m[v] = struct{}{}
		}

		ret := []string{}

		for _, v := range p2 {
			if _, ok := m[v]; ok {
				ret = append(ret, v)
			}
		}

		return ret
	}
}

type libInfo struct {
	Name        string
	Version     string
	RepoVersion string
	BinaryLibs  string
	HaveInit    bool
	InitFunc    string
	Deps        []string
}

type moduleInfo struct {
	Name        string
	RepoVersion string
}

type libsInitData struct {
	Libs    []libInfo
	Modules []moduleInfo
}

func quoteOrNULL(s string) string {
	if s == "" {
		return "NULL"
	}
	return fmt.Sprintf("%q", s)
}

func getDepsInitCCode(manifest *build.FWAppManifest, dm *build.DepsManifest) ([]byte, error) {
	tplData := libsInitData{}
	for _, n := range manifest.InitDeps {
		var lh *build.FWAppManifestLibHandled
		for _, lh1 := range manifest.LibsHandled {
			if lh1.Lib.Name == n {
				lh = &lh1
				break
			}
		}
		initFunc := "NULL"
		if len(lh.Sources) > 0 || len(lh.BinaryLibs) > 0 {
			initFunc = fmt.Sprintf("mgos_%s_init", ourutil.IdentifierFromString(lh.Lib.Name))
		}
		var blHashes []string
		for _, dl := range dm.Libs {
			if dl.Name != lh.Lib.Name {
				continue
			}
			for _, bl := range dl.Blobs {
				blHashes = append(blHashes, fmt.Sprintf("%s:%x", bl.Name, bl.SHA256))
			}
			break
		}
		rv := lh.RepoVersion
		if rv != "" && lh.RepoDirty {
			rv = fmt.Sprintf("%s-dirty", lh.RepoVersion)
		}
		tplData.Libs = append(tplData.Libs, libInfo{
			Name:        quoteOrNULL(lh.Lib.Name),
			Version:     quoteOrNULL(lh.UserVersion),
			RepoVersion: quoteOrNULL(rv),
			BinaryLibs:  quoteOrNULL(strings.Join(blHashes, ",")),
			HaveInit:    initFunc != "NULL",
			InitFunc:    initFunc,
			Deps:        lh.InitDeps,
		})
	}

	for _, m := range manifest.Modules {
		mrv := ""
		if lmrv, dirty, err := m.GetRepoVersion(); err == nil {
			if lmrv != "" && dirty {
				mrv = fmt.Sprintf("%s-dirty", lmrv)
			} else {
				mrv = lmrv
			}
		}
		tplData.Modules = append(tplData.Modules, moduleInfo{
			Name:        quoteOrNULL(m.Name),
			RepoVersion: quoteOrNULL(mrv),
		})
	}

	tpl := template.Must(template.New("depsInit").Parse(
		string(MustAsset("data/mgos_deps_init.c.tmpl")),
	))

	var c bytes.Buffer
	if err := tpl.Execute(&c, tplData); err != nil {
		return nil, errors.Trace(err)
	}

	return c.Bytes(), nil
}

// resolvePaths takes a list of paths as they are in manifest, globs like
// []string{"*.c", "*.h"}, and converts those paths into paths to concrete
// existing files.
//
// There are three kinds of paths which can be present in the input srcPaths:
// - Globs, like "foo/bar/*.c". Those get expanded to the list of concrete files.
// - Paths to dirs. Those get appended all the given globs, and then treated
//   as the globs above
// - Paths to concrete files. Those stay unchanged.
//
// Paths in srcPaths can be prefixed with a `+` (which is a no-op) or with `-`
// (which excludes matching files from the result). E.g. []string{"foo",
// "-foo/bar"} means "all files under foo, except foo/bar".
func resolvePaths(srcPaths []string, globs []string) (files []string, dirs []string, err error) {
	// Get separate slices of paths to add and paths to remove
	add := []string{}
	remove := []string{}

	for _, g := range srcPaths {
		if g[0] == '-' {
			remove = append(remove, g[1:])
		} else if g[0] == '+' {
			add = append(add, g[1:])
		} else {
			add = append(add, g)
		}
	}

	// Get slice of concrete files to add and to remove
	addFiles, addDirs, err := resolvePathsUnprefixed(add, globs)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	removeFiles, removeDirs, err := resolvePathsUnprefixed(remove, globs)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	// Actually remove files-to-remove from files-to-add
	removeFilesMap := map[string]struct{}{}
	removeDirsMap := map[string]struct{}{}

	for _, v := range removeFiles {
		removeFilesMap[v] = struct{}{}
	}

	for _, v := range removeDirs {
		removeDirsMap[v] = struct{}{}
	}

	addFilesOrig := addFiles
	addDirsOrig := addDirs

	addFiles = []string{}
	addDirs = []string{}

	for _, v := range addFilesOrig {
		if _, ok := removeFilesMap[v]; !ok {
			addFiles = append(addFiles, v)
		}
	}

	for _, v := range addDirsOrig {
		if _, ok := removeDirsMap[v]; !ok {
			addDirs = append(addDirs, v)
		}
	}

	return addFiles, addDirs, nil
}

// resolvePathsUnprefixed is like resolvePaths, but doesn't support
// `-` and `+` as filename prefixes.
func resolvePathsUnprefixed(srcPaths []string, globs []string) (files []string, dirs []string, err error) {
	var fileGlobs []string
	fileGlobs, dirs, err = globify(srcPaths, globs)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	for _, g := range fileGlobs {
		matches, err := filepath.Glob(g)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}

		files = append(files, matches...)
	}

	return files, dirs, nil
}

// globify takes a list of paths, and for each of them which resolves to a
// directory adds each glob from provided globs. Other paths are added as they
// are.
func globify(srcPaths []string, globs []string) (sources []string, dirs []string, err error) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	for _, p := range srcPaths {
		p = filepath.FromSlash(p)
		finfo, err := os.Stat(p)
		var curDir string
		if err == nil && finfo.IsDir() {
			// Item exists and is a directory; add given globs to it
			for _, glob := range globs {
				sources = append(sources, filepath.Join(p, glob))
			}
			curDir = p
		} else {
			if err != nil {
				// Item either does not exist or is a glob
				if !os.IsNotExist(errors.Cause(err)) && runtime.GOOS != "windows" {
					// Some error other than non-existing file, return an error (on
					// Windows, path with glob result in some other error like malformed
					// path, so on windows we can't distinguish kinds of errors)
					return nil, nil, errors.Trace(err)
				}

				// Try to interpret current item as a glob; if it does not resolve
				// to anything, we'll silently ignore it
				matches, err := filepath.Glob(p)
				if err != nil {
					return nil, nil, errors.Trace(err)
				}

				if len(matches) == 0 {
					// The item did not resolve to anything when interpreted as a glob,
					// assume it does not exist, and silently ignore
					continue
				}
			}

			// Item is an existing file or a glob which resolves to something; just
			// add it as it is
			sources = append(sources, p)
			curDir = filepath.Dir(p)
		}
		d, err := filepath.Abs(curDir)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		dirs = append(dirs, d)
	}

	// We want source paths to be absolute, but sources are globs, so we can't do
	// filepath.Abs on it. Instead, we can just do filepath.Join(cwd, s) if
	// the path is not absolute.
	for k, s := range sources {
		if !filepath.IsAbs(s) {
			sources[k] = filepath.Join(cwd, s)
		}
	}

	return sources, dirs, nil
}

func getAllSupportedPlatforms(mosDir string) ([]string, error) {
	ret := strings.Split(supportedPlatforms, " ")
	sort.Strings(ret)
	return ret, nil
}
