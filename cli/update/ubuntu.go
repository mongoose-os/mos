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
	"bufio"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"github.com/juju/errors"

	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/version"
)

var (
	ubuntuPackageNames = map[UpdateChannel]string{
		UpdateChannelRelease: "mos",
		UpdateChannelLatest:  "mos-latest",
	}
	ubuntuRepoName      = "ppa:mongoose-os/mos"
	ubuntuDEBURLPrefix  = "https://mongoose-os.com/downloads/debs/"
	ubuntuDEBArchiveURL = ubuntuDEBURLPrefix + "index.txt"
	ubuntuDEBNameRegexp = regexp.MustCompile(`^(?P<pkg>[^_]+)_(?P<version>[^+]+)\+(?P<hash>[^~]+)\~(?P<distr>[^\d]+)\d+_(?P<arch>[^.]+)\.deb`)
)

func doUbuntuUpdateRepo(oldUpdChannel, newUpdChannel UpdateChannel) error {
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

func doUbuntuUpdateDEB(oldUpdChannel UpdateChannel, newMosVersion string) error {
	distr := version.GetUbuntuBuildIDParts(version.BuildId)["distr"]
	arch := runtime.GOARCH
	if arch == "arm" {
		arch = "armhf"
	}
	glog.Infof("Distr: %s, arch: %s", distr, arch)
	debName := ""
	{
		glog.Infof("Fetching package archive index from %s...", ubuntuDEBArchiveURL)
		resp, err := http.Get(ubuntuDEBArchiveURL)
		if err != nil {
			return errors.Annotatef(err, "failed to fetch package archive index")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("failed to fetch package archive index from %s: %s", ubuntuDEBArchiveURL, resp.Status)
		}
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			d := strings.Split(scanner.Text(), " ")[0]
			p := ourutil.FindNamedSubmatches(ubuntuDEBNameRegexp, scanner.Text())
			glog.V(2).Infof("%s -> %s", scanner.Text(), p)
			if p["distr"] == distr && p["version"] == newMosVersion && p["arch"] == arch {
				debName = d
				break
			}
		}
		if debName == "" {
			return errors.Errorf("no package found for %s %s %s", newMosVersion, distr, arch)
		}
	}
	debURL := ubuntuDEBURLPrefix + debName
	ourutil.Reportf("Downloading %s...", debURL)
	resp, err := http.Get(debURL)
	if err != nil {
		return errors.Annotatef(err, "failed to fetch package")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("failed to fetch package from %s: %s", debURL, resp.Status)
	}
	debData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Annotatef(err, "failed to fetch package")
	}
	tempDir, err := ioutil.TempDir("", "mosdeb-")
	if err != nil {
		return errors.Annotatef(err, "failed to create staging directory")
	}
	if !*flags.KeepTempFiles {
		defer os.RemoveAll(tempDir)
	}
	debFile := filepath.Join(tempDir, debName)
	if err = ioutil.WriteFile(debFile, debData, 0644); err != nil {
		return errors.Annotatef(err, "failed to write package file")
	}
	glog.Infof("Wrote %s %s", debFile, resp.Status)
	ourutil.Reportf("Removing installed package (if any)...")
	oldPkg := ubuntuPackageNames[oldUpdChannel]
	ourutil.RunCmd(ourutil.CmdOutOnError, "sudo", "apt-get", "remove", "-y", oldPkg)
	ourutil.Reportf("Installing new package...")
	if err = ourutil.RunCmd(ourutil.CmdOutOnError, "sudo", "dpkg", "-i", debFile); err != nil {
		return errors.Annotatef(err, "failed to install %s", debFile)
	}
	ourutil.Reportf("Package installed")
	return nil
}
