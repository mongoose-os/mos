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
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/juju/errors"
	"github.com/kardianos/osext"
	goversion "github.com/mcuadros/go-version"
	flag "github.com/spf13/pflag"

	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/common/state"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/version"
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
func GetServerMosVersion(mosVersion string, extraInfo ...string) (*version.VersionJson, error) {
	client := &http.Client{}
	versionUrl := getMosURL("version.json", mosVersion)
	req, err := http.NewRequest("GET", versionUrl, nil)
	if extraInfo != nil {
		req.Header.Add("User-Agent", fmt.Sprintf("%s; %s", version.GetUserAgent(), strings.Join(extraInfo, "; ")))
	} else {
		req.Header.Add("User-Agent", version.GetUserAgent())
	}
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
		if newMosVersion == string(newUpdChannel) {
			return doUbuntuUpdateRepo(updChannel, newUpdChannel)
		} else {
			return doUbuntuUpdateDEB(updChannel, newMosVersion)
		}
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

		stateVer = &state.StateVersion{}
		state.SetStateForVersion(mosVersion, stateVer)

		if err := state.SaveState(); err != nil {
			return errors.Trace(err)
		}

		ourutil.Reportf("Init done.")
	}

	return nil
}
