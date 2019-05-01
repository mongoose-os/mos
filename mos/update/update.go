package update

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"context"

	"cesanta.com/common/go/ourgit"
	"cesanta.com/common/go/ourio"
	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/build"
	moscommon "cesanta.com/mos/common"
	"cesanta.com/mos/common/paths"
	"cesanta.com/mos/common/state"
	"cesanta.com/mos/mosgit"
	"cesanta.com/mos/version"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	"github.com/kardianos/osext"
	goversion "github.com/mcuadros/go-version"
	flag "github.com/spf13/pflag"

	"cesanta.com/mos/dev"
)

const (
	UpdateChannelLatest  UpdateChannel = "latest"
	UpdateChannelRelease               = "release"
)

var (
	migrateFlag = flag.Bool("migrate", true, "Migrate data from the previous version if needed")

	brewPackageNames = map[UpdateChannel]string{
		UpdateChannelRelease: "mos",
		UpdateChannelLatest:  "mos-latest",
	}
	ubuntuPackageNames = map[UpdateChannel]string{
		UpdateChannelRelease: "mos",
		UpdateChannelLatest:  "mos-latest",
	}
	ubuntuRepoName = "ppa:mongoose-os/mos"
)

// mosVersion can be either exact mos version like "1.6", or update channel
// like "latest" or "release".
func getMosURL(p, mosVersion string) string {
	return "https://" + path.Join(
		fmt.Sprintf("mongoose-os.com/downloads/mos%s", moscommon.GetVersionSuffix(mosVersion)),
		p,
	)
}

// mosVersion can be either exact mos version like "1.6", or update channel
// like "latest" or "release".
func GetServerMosVersion(mosVersion string) (*version.VersionJson, error) {
	client := &http.Client{}
	versionUrl := getMosURL("version.json", mosVersion)
	req, err := http.NewRequest("GET", versionUrl, nil)
	req.Header.Add("User-Agent", version.GetUserAgent())
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("got %d when accessing %s", resp.StatusCode, versionUrl)
	}

	defer resp.Body.Close()

	var serverVersion version.VersionJson

	decoder := json.NewDecoder(resp.Body)
	decoder.Decode(&serverVersion)

	return &serverVersion, nil
}

func doUbuntuUpdate(oldUpdChannel, newUpdChannel UpdateChannel) error {
	oldPkg := ubuntuPackageNames[oldUpdChannel]
	newPkg := ubuntuPackageNames[newUpdChannel]

	// Start with an apt-get update.
	// Do not fail because some unrelated repo may be screwed up
	// but our PPA might still be ok.
	ourutil.RunCmd(ourutil.CmdOutOnError, "sudo", "apt-get", "update")

	// Check if mos and mos-latest are among the available packages.
	output, err := ourutil.GetCommandOutput("apt-cache", "showpkg", newPkg)
	if err != nil {
		return errors.Annotatef(err, "faild to get package info")
	}
	if !strings.Contains(output, "/lists/") {
		// No package info in repo lists - we should (re-)add our repo.
		// This can happen, for example, after release upgrade which disables 3rd-party repos.
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, "sudo", "apt-add-repository", "-y", "-u", ubuntuRepoName); err != nil {
			return errors.Trace(err)
		}
	}

	if oldPkg != newPkg {
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, "sudo", "apt-get", "remove", "-y", oldPkg); err != nil {
			return errors.Trace(err)
		}
	}
	return ourutil.RunCmd(ourutil.CmdOutAlways, "sudo", "apt-get", "install", "-y", newPkg)
}

func doBrewUpdate(oldUpdChannel, newUpdChannel UpdateChannel) error {
	oldPkg := brewPackageNames[oldUpdChannel]
	newPkg := brewPackageNames[newUpdChannel]
	ourutil.RunCmd(ourutil.CmdOutOnError, "brew", "update")
	ourutil.RunCmd(ourutil.CmdOutOnError, "brew", "tap", "cesanta/mos")
	if oldPkg != newPkg {
		ourutil.RunCmd(ourutil.CmdOutOnError, "brew", "uninstall", "-f", oldPkg)
	}
	return ourutil.RunCmd(ourutil.CmdOutAlways, "brew", "install", newPkg)
}

func Update(ctx context.Context, devConn dev.DevConn) error {
	args := flag.Args()

	// updChannel and newUpdChannel are needed for the logging, so that it's
	// clear for the user which update channel is used
	updChannel := GetUpdateChannel()
	newUpdChannel := updChannel

	// newMosVersion is the version which will be fetched from the server;
	// by default it's equal to the current update channel.
	newMosVersion := string(updChannel)

	if len(args) >= 2 {
		// Desired mos version is given
		newMosVersion = args[1]
		newUpdChannel = getUpdateChannelByMosVersion(newMosVersion)
	}

	if updChannel != newUpdChannel {
		ourutil.Reportf("Changing update channel from %q to %q", updChannel, newUpdChannel)
	} else {
		ourutil.Reportf("Update channel: %s", updChannel)
	}

	if version.LooksLikeUbuntuBuildId(version.BuildId) {
		return doUbuntuUpdate(updChannel, newUpdChannel)
	} else if version.LooksLikeBrewBuildId(version.BuildId) {
		return doBrewUpdate(updChannel, newUpdChannel)
	} else if version.LooksLikeDistrBuildId(version.BuildId) {
		ourutil.Reportf("Mos was installed via some package manager, please use it to update.")
		return nil
	}

	var mosUrls = map[string]string{
		"windows": getMosURL("win/mos.exe", newMosVersion),
		"linux":   getMosURL("linux/mos", newMosVersion),
		"darwin":  getMosURL("mac/mos", newMosVersion),
	}

	// Check the available version on the server
	serverVersion, err := GetServerMosVersion(newMosVersion)
	if err != nil {
		return errors.Trace(err)
	}

	if serverVersion.BuildId != version.BuildId {
		// Versions are different, perform update
		ourutil.Reportf("Current version: %s, available version: %s.",
			version.BuildId, serverVersion.BuildId,
		)

		// Determine the right URL for the current platform
		mosUrl, ok := mosUrls[runtime.GOOS]
		if !ok {
			keys := make([]string, len(mosUrls))

			i := 0
			for k := range mosUrls {
				keys[i] = k
				i++
			}

			return errors.Errorf("unsupported OS: %s (supported values are: %v)",
				runtime.GOOS, keys,
			)
		}

		// Create temp file to save downloaded data into
		// (we should create it in the same dir as the executable to be updated,
		// just in case /tmp and executable are on different devices)
		executableDir, err := osext.ExecutableFolder()
		if err != nil {
			return errors.Trace(err)
		}

		tmpfile, err := ioutil.TempFile(executableDir, "mos_update_")
		if err != nil {
			return errors.Trace(err)
		}
		defer tmpfile.Close()

		// Fetch data from the server and save it into the temp file
		resp, err := http.Get(mosUrl)
		if err != nil {
			return errors.Trace(err)
		}
		defer resp.Body.Close()

		ourutil.Reportf("Downloading from %s...", mosUrl)
		n, err := io.Copy(tmpfile, resp.Body)
		if err != nil {
			return errors.Trace(err)
		}

		// Check saved length
		if n != resp.ContentLength {
			return errors.Errorf("expected to write %d bytes, %d written",
				resp.ContentLength, n,
			)
		}
		tmpfile.Close()

		// Determine names for the executable and backup
		executable, err := osext.Executable()
		if err != nil {
			return errors.Trace(err)
		}

		bak := fmt.Sprintf("%s.bak", executable)

		ourutil.Reportf("Renaming old binary as %s...", bak)
		if err := os.Rename(executable, bak); err != nil {
			return errors.Trace(err)
		}

		ourutil.Reportf("Saving new binary as %s...", executable)
		if err := os.Rename(tmpfile.Name(), executable); err != nil {
			return errors.Trace(err)
		}

		// Make sure the new binary is, indeed, executable
		if err := os.Chmod(executable, 0755); err != nil {
			return errors.Trace(err)
		}

		ourutil.Reportf("Done.")
	} else {
		ourutil.Reportf("Up to date.")
	}

	return nil
}

// GetUpdateChannel returns update channel (either "latest" or "release")
// depending on current mos version.
func GetUpdateChannel() UpdateChannel {
	return getUpdateChannelByMosVersion(version.GetMosVersion())
}

type UpdateChannel string

// getUpdateChannelByMosVersion returns update channel (either "latest" or
// "release") depending on the given mos version.
func getUpdateChannelByMosVersion(mosVersion string) UpdateChannel {
	if mosVersion == "master" || mosVersion == "latest" {
		return UpdateChannelLatest
	}
	return UpdateChannelRelease
}

func Init() error {
	if *migrateFlag {
		if err := migrateData(); err != nil {
			// Just print the error
			fmt.Println(err.Error())
		}
	}

	return nil
}

// migrateData converts old single libs/apps/modules dirs (if they are present)
// to the new per-version shape, and then checks in state.json whether current
// version already has imported libs from previous version. If not, then
// performs the import.
func migrateData() error {
	mosVersion := version.GetMosVersion()

	convertedVersions := []string{}

	if len(convertedVersions) > 0 {
		// We've converted some old dir(s) into the new versioned shape, let's
		// write the latest version as the "initialized" one, so we could
		// copy state from it
		goversion.Sort(convertedVersions)
		latestConverted := convertedVersions[len(convertedVersions)-1]

		if state.GetStateForVersion(latestConverted) == nil {
			state.SetStateForVersion(latestConverted, &state.StateVersion{})
			if err := state.SaveState(); err != nil {
				return errors.Trace(err)
			}
		}
	}

	// Latest version is special, it doesn't import libs from other versions
	if mosVersion == "latest" {
		return nil
	}

	stateVer := state.GetStateForVersion(mosVersion)
	if stateVer == nil {
		// Need to initialize current version

		ourutil.Reportf("First run of the version %s, initializing...", mosVersion)

		// Get sorted list of all versions available
		versions := []string{}
		for k, _ := range state.GetState().Versions {
			versions = append(versions, k)
		}
		goversion.Sort(versions)

		if len(versions) > 0 {
			// There are some versions available, so we'll pick the latest one
			// and copy data from it to the current version
			latestVersion := versions[len(versions)-1]

			if err := migrateProjects(paths.AppsDirTpl, latestVersion, mosVersion); err != nil {
				return errors.Trace(err)
			}
		} else {
			// No other versions available, so nothing to do
		}

		stateVer = &state.StateVersion{}
		state.SetStateForVersion(mosVersion, stateVer)

		if err := state.SaveState(); err != nil {
			return errors.Trace(err)
		}

		ourutil.Reportf("Init done.")
	}

	return nil
}

// migrateProjects migrates all projects from the given oldVer to newVer,
// in the directory determined by the given template dirTpl (like ~/.mos/libs-${mos.version})
// All projects are migrated in parallel
func migrateProjects(dirTpl, oldVer, newVer string) error {
	oldDir, err := paths.NormalizePath(dirTpl, oldVer)
	if err != nil {
		return errors.Trace(err)
	}

	newDir, err := paths.NormalizePath(dirTpl, newVer)
	if err != nil {
		return errors.Trace(err)
	}

	if _, err := os.Stat(newDir); err == nil {
		// Target dir already exists, do nothing
		return nil
	}

	entries, err := ioutil.ReadDir(oldDir)
	if err != nil {
		// Ignore errors; the dir might just not exist, and we don't care much
		return nil
	}

	// We migrate all dirs in parallel, and we just print errors, because we
	// don't care much about them
	wg := &sync.WaitGroup{}
	for _, entry := range entries {
		wg.Add(1)
		go migrateProj(
			filepath.Join(oldDir, entry.Name()),
			filepath.Join(newDir, entry.Name()),
			oldVer,
			wg,
		)
	}
	wg.Wait()

	return nil
}

// migrateProj migrates a single project from oldDir to newDir; from the
// given version oldVer to the current mos version.
func migrateProj(oldDir, newDir, oldVer string, wg *sync.WaitGroup) {
	defer wg.Done()

	gitinst := mosgit.NewOurGit()

	glog.Infof("Copying %s as %s...", oldDir, newDir)
	if err := ourio.CopyDir(oldDir, newDir, nil); err != nil {
		ourutil.Reportf("Error copying %s as %s: %s", oldDir, newDir, err)
	}

	projBase := filepath.Base(newDir)
	projDir := filepath.Dir(newDir)

	basename, projectVersion, _ := parseProjectDirname(projBase)

	if projectVersion == oldVer {
		originURL, err := gitinst.GetOriginUrl(newDir)
		if err != nil {
			ourutil.Reportf("Failed to get git origin for %s", newDir)
			return
		}

		oldNewDir := newDir
		newDir = filepath.Join(
			projDir,
			fmt.Sprint(basename, moscommon.GetVersionSuffix(version.GetMosVersion())),
		)
		os.Rename(oldNewDir, newDir)

		logWriter := bytes.Buffer{}

		swmod := build.SWModule{
			Location: originURL,
			Version:  version.GetMosVersion(),
		}

		glog.Infof("Checking out %s at the version %s...", basename, version.GetMosVersion())
		_, err = swmod.PrepareLocalDir(filepath.Dir(newDir), &logWriter, true, "", time.Duration(0), 0)
		if err != nil {
			ourutil.Reportf("Error preparing local dir for %s: %s", newDir, err)
		}

		ourutil.Reportf("Imported %s", projBase)

	} else {
		glog.Infof("Leaving %s intact because the version %s is not equal to %s", basename, projectVersion, oldVer)
	}
}

// parseProjectDirname takes the dir name like "foo-bar-1.12" and tries to
// guess the actual project name, corresponding mos version and library
// version. E.g. for "foo-bar-1.12" it will return "foo-bar", "1.12", "1.12".
//
// It checks the suffix and figures whether it looks like a version name or not.
// Valid versions are strings like "1.12", "latest", "release", and git SHA
// if it matches the actual current SHA.
//
// dirVersion might differ from projectVersion if only the suffix is a valid
// git SHA, in which case dirVersion will be "latest".
func parseProjectDirname(projectDir string) (basename, projectVersion, dirVersion string) {
	gitinst := mosgit.NewOurGit()

	projectDirBase := filepath.Base(projectDir)
	parts := strings.Split(projectDirBase, "-")

	if len(parts) == 1 {
		// No suffix, assume "latest"
		basename = parts[0]
		projectVersion = "latest"
		dirVersion = projectVersion
	} else {
		// Initially assume the last part is the version; we'll check below if
		// it's really the case
		projectVersion = parts[len(parts)-1]
		dirVersion = projectVersion
		basename = strings.Join(parts[:len(parts)-1], "-")

		if !version.LooksLikeVersionNumber(projectVersion) && projectVersion != "latest" && projectVersion != "release" {
			// Suffix does not look like a version, but let's check if it's a SHA
			sha, err := gitinst.GetCurrentHash(projectDir)
			if err == nil {
				if ourgit.HashesEqual(projectVersion, sha) {
					// Yes it's a SHA. We don't really know in which dir to put it,
					// so we'll put in latest
					dirVersion = "latest"
				} else {
					// No, it's not a SHA either, so assume "latest"
					basename = projectDirBase
					projectVersion = "latest"
					dirVersion = projectVersion
				}
			} else {
				basename = projectDirBase
				projectVersion = "latest"
				dirVersion = projectVersion
			}
		}
	}

	return
}
