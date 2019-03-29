package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"context"

	"cesanta.com/common/go/fwbundle"
	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/build"
	moscommon "cesanta.com/mos/common"
	"cesanta.com/mos/common/paths"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"
	"cesanta.com/mos/interpreter"
	"cesanta.com/mos/manifest_parser"
	"cesanta.com/mos/mosgit"
	"cesanta.com/mos/update"
	"cesanta.com/mos/version"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"
)

// mos build specific advanced flags
var (
	buildImageFlag     = flag.String("build-image", "", "Override the Docker image used for build.")
	cleanBuildFlag     = flag.Bool("clean", false, "perform a clean build, wipe the previous build state")
	buildDryRunFlag    = flag.Bool("build-dry-run", false, "do not actually run the build, only prepare")
	buildParamsFlag    = flag.String("build-params", "", "build params file")
	buildTarget        = flag.String("build-target", moscommon.BuildTargetDefault, "target to build with make")
	keepTempFiles      = flag.Bool("keep-temp-files", false, "keep temp files after the build is done (by default they are in ~/.mos/tmp)")
	modules            = flag.StringArray("module", []string{}, "location of the module from mos.yaml, in the format: \"module_name:/path/to/location\". Can be used multiple times.")
	libs               = flag.StringArray("lib", []string{}, "location of the lib from mos.yaml, in the format: \"lib_name:/path/to/location\". Can be used multiple times.")
	libsUpdateInterval = flag.Duration("libs-update-interval", time.Minute*30, "how often to update already fetched libs")

	buildDockerExtra = flag.StringArray("build-docker-extra", []string{}, "extra docker flags, added before image name. Can be used multiple times: e.g. --build-docker-extra -v --build-docker-extra /foo:/bar.")
	buildCmdExtra    = flag.StringArray("build-cmd-extra", []string{}, "extra make flags, added at the end of the make command. Can be used multiple times.")
	cflagsExtra      = flag.StringArray("cflags-extra", []string{}, "extra C flag, appended to the \"cflags\" in the manifest. Can be used multiple times.")
	cxxflagsExtra    = flag.StringArray("cxxflags-extra", []string{}, "extra C++ flag, appended to the \"cxxflags\" in the manifest. Can be used multiple times.")
	libsExtraFlag    = flag.StringArray("lib-extra", []string{}, "Extra libs to add to the app being built. Value should be a YAML string. Can be used multiple times.")
	buildParalellism = flag.Int("build-parallelism", 0, "build parallelism. default is to use number of CPUs.")
	saveBuildStat    = flag.Bool("save-build-stat", true, "save build statistics")

	noPlatformCheckFlag = flag.Bool("no-platform-check", false, "override platform support check")

	preferPrebuiltLibs = flag.Bool("prefer-prebuilt-libs", false, "if both sources and prebuilt binary of a lib exists, use the binary")

	buildVarsSlice = flag.StringSlice("build-var", []string{}, `Build variable in the format "NAME=VALUE". Can be used multiple times.`)
	cdefsSlice     = flag.StringSlice("cdef", []string{}, `C/C++ define in the format "NAME=VALUE". Can be used multiple times.`)

	noLibsUpdate  = flag.Bool("no-libs-update", false, "if true, never try to pull existing libs (treat existing default locations as if they were given in --lib)")
	skipCleanLibs = flag.Bool("skip-clean-libs", true, "if false, then during the remote build all libs will be uploaded to the builder")

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

type buildParams struct {
	manifest_parser.ManifestAdjustments
	BuildTarget           string
	CustomLibLocations    map[string]string
	CustomModuleLocations map[string]string
	NoPlatformCheck       bool
}

func init() {
	hiddenFlags = append(hiddenFlags, "docker_images")
}

// Build {{{

// Build command handler {{{
func buildHandler(ctx context.Context, devConn dev.DevConn) error {
	var bParams buildParams
	if *buildParamsFlag != "" {
		buildParamsBytes, err := ioutil.ReadFile(*buildParamsFlag)
		if err != nil {
			return errors.Annotatef(err, "error reading --build-params file")
		}
		if err := yaml.Unmarshal(buildParamsBytes, &bParams); err != nil {
			return errors.Annotatef(err, "error parsing --build-params file")
		}
	} else {
		// Create map of given lib locations, via --lib flag(s)
		cll, err := getCustomLibLocations()
		if err != nil {
			return errors.Trace(err)
		}

		// Create map of given module locations, via --module flag(s)
		cml := map[string]string{}
		for _, m := range *modules {
			parts := strings.SplitN(m, ":", 2)
			cml[parts[0]] = parts[1]
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

		bParams = buildParams{
			ManifestAdjustments: manifest_parser.ManifestAdjustments{
				Platform:  flags.Platform(),
				BuildVars: buildVarsFromCLI,
				CDefs:     cdefsFromCLI,
				CFlags:    *cflagsExtra,
				CXXFlags:  *cxxflagsExtra,
				ExtraLibs: libsFromCLI,
			},
			BuildTarget:           *buildTarget,
			CustomLibLocations:    cll,
			CustomModuleLocations: cml,
			NoPlatformCheck:       *noPlatformCheckFlag,
		}
	}

	return errors.Trace(doBuild(ctx, &bParams))
}

func doBuild(ctx context.Context, bParams *buildParams) error {
	var err error
	buildDir := moscommon.GetBuildDir(projectDir)

	if bParams.BuildTarget == "" {
		bParams.BuildTarget = moscommon.BuildTargetDefault
	}

	start := time.Now()

	// Request server version in parallel
	serverVersionCh := make(chan *version.VersionJson, 1)
	if true || !*local {
		go func() {
			v, err := update.GetServerMosVersion(string(update.GetUpdateChannel()))
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

	if *verbose {
		logWriter = logWriterStderr
	}

	// Fail fast if there is no manifest
	if _, err := os.Stat(moscommon.GetManifestFilePath(projectDir)); os.IsNotExist(err) {
		return errors.Errorf("No mos.yml file")
	}

	if *local {
		err = buildLocal(ctx, bParams)
	} else {
		err = buildRemote(bParams)
	}
	if err != nil {
		return errors.Trace(err)
	}
	if *buildDryRunFlag {
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

		if *saveBuildStat {
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

		if *local || !*verbose {
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
	if err := parseVarsSlice(*buildVarsSlice, m); err != nil {
		return nil, errors.Annotatef(err, "invalid --build-var")
	}
	return m, nil
}

func getCdefsFromCLI() (map[string]string, error) {
	m := map[string]string{}
	if err := parseVarsSlice(*cdefsSlice, m); err != nil {
		return nil, errors.Annotatef(err, "invalid --cdef")
	}
	return m, nil
}

func getLibsFromCLI() ([]build.SWModule, error) {
	var res []build.SWModule
	for _, v := range *libsExtraFlag {
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

func getCustomLibLocations() (map[string]string, error) {
	customLibLocations := map[string]string{}
	for _, l := range *libs {
		parts := strings.SplitN(l, ":", 2)

		// Absolutize the given lib path
		var err error
		parts[1], err = filepath.Abs(parts[1])
		if err != nil {
			return nil, errors.Trace(err)
		}

		customLibLocations[parts[0]] = parts[1]
	}
	return customLibLocations, nil
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
	bParams   *buildParams
	logWriter io.Writer
}

func (lpr *compProviderReal) GetLibLocalPath(
	m *build.SWModule, rootAppDir, libsDefVersion, platform string,
) (string, error) {
	gitinst := mosgit.NewOurGit()

	name, err := m.GetName()
	if err != nil {
		return "", errors.Trace(err)
	}

	appDir, err := getCodeDirAbs()
	if err != nil {
		return "", errors.Trace(err)
	}

	// --lib has the highest precedence.
	libDirAbs, ok := lpr.bParams.CustomLibLocations[name]
	if ok {
		ourutil.Freportf(lpr.logWriter, "%s: Using %q (--lib)", name, libDirAbs)
		return libDirAbs, nil
	}

	// If --libs-dir is set, this is where all the libs are.
	if paths.LibsDirFlag != "" {
		name2, _ := m.GetName2()
		libDirAbs := filepath.Join(paths.LibsDirFlag, name2)
		ourutil.Freportf(lpr.logWriter, "%s: Using %q (--libs-dir) %s", name, libDirAbs, name2)
		return libDirAbs, nil
	}

	// Try to fetch
	depsDir := paths.GetDepsDir(appDir)
	for {
		localDir, err := m.GetLocalDir(depsDir, libsDefVersion)
		if err != nil {
			return "", errors.Trace(err)
		}

		updateIntvl := *libsUpdateInterval
		if *noLibsUpdate {
			updateIntvl = 0
		}

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

		if m.GetType() == build.SWModuleTypeGit {
			if newHash, err := gitinst.GetCurrentHash(localDir); err == nil && newHash != curHash {
				freportf(logWriter, "%s: Hash is updated: %s -> %s", name, curHash, newHash)
				// The current repo hash has changed after the pull, so we need to
				// vanish binary lib(s) we might have downloaded before
				bLibs, _ := filepath.Glob(moscommon.GetBinaryLibFilePath(moscommon.GetBuildDir(appDir), name, "*", "*"))
				for _, bl := range bLibs {
					os.Remove(bl)
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

	targetDir, ok := lpr.bParams.CustomModuleLocations[name]
	if !ok {
		// Custom module location wasn't provided in command line, so, we'll
		// use the module name and will clone/pull it if necessary
		freportf(logWriter, "The flag --module is not given for the module %q, going to use the repository", name)

		appDir, err := getCodeDirAbs()
		if err != nil {
			return "", errors.Trace(err)
		}

		updateIntvl := *libsUpdateInterval
		if *noLibsUpdate {
			updateIntvl = 0
		}

		targetDir, err = m.PrepareLocalDir(paths.GetModulesDir(appDir), logWriter, true, modulesDefVersion, updateIntvl, 0)
		if err != nil {
			return "", errors.Annotatef(err, "preparing local copy of the module %q", name)
		}
	} else {
		freportf(logWriter, "Using module %q located at %q", name, targetDir)
	}

	return targetDir, nil
}

func (lpr *compProviderReal) GetMongooseOSLocalPath(
	rootAppDir, modulesDefVersion string,
) (string, error) {
	targetDir, err := getMosDirEffective(modulesDefVersion, *libsUpdateInterval)
	if err != nil {
		return "", errors.Trace(err)
	}

	return targetDir, nil
}

func getMosDirEffective(mongooseOsVersion string, updateInterval time.Duration) (string, error) {
	var mosDirEffective string
	if *mosRepo != "" {
		freportf(logWriter, "Using mongoose-os located at %q", *mosRepo)
		mosDirEffective = *mosRepo
	} else {
		freportf(logWriter, "The flag --repo is not given, going to use mongoose-os repository")
		appDir, err := getCodeDirAbs()
		if err != nil {
			return "", errors.Trace(err)
		}

		md := paths.GetModulesDir(appDir)

		m := build.SWModule{
			// TODO(dfrank) get upstream repo URL from a flag
			// (and this flag needs to be forwarded to fwbuild as well, which should
			// forward it to the mos invocation)
			Location: "https://github.com/cesanta/mongoose-os",
			Version:  mongooseOsVersion,
		}

		if *noLibsUpdate {
			updateInterval = 0
		}

		if mosDirEffective == "" {
			// NOTE: mongoose-os repo is huge, so in order to save space and time, we
			// do a shallow clone (--depth 1).
			mosDirEffective, err = m.PrepareLocalDir(md, logWriter, true, "", updateInterval, 1)
			if err != nil {
				return "", errors.Annotatef(err, "preparing local copy of the mongoose-os repo")
			}
		}
	}

	return mosDirEffective, nil
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
