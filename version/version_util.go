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
package version

// version.go is generated separately in Makefile
// to avoid update during "blanket" go generate runs

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	moscommon "github.com/mongoose-os/mos/cli/common"
	"github.com/mongoose-os/mos/cli/ourutil"
)

type VersionJson struct {
	BuildId        string    `json:"build_id"`
	BuildTimestamp time.Time `json:"build_timestamp"`
	BuildVersion   string    `json:"build_version"`
}

const (
	LatestVersionName = "latest"
)

var (
	regexpVersionNumber = regexp.MustCompile(`^\d+\.[0-9.]*$`)
	regexpBuildIdDistr  = regexp.MustCompile(`^(?P<version>[^+]+)\+(?P<hash>[^~]+)\~(?P<distr>[^\d]+)\d+$`)

	ubuntuDistrNames = []string{"xenial", "bionic", "disco", "eoan"}
)

// GetMosVersion returns this binary's version, or "latest" if it's not a release build.
func GetMosVersion() string {
	if LooksLikeVersionNumber(Version) {
		return Version
	}
	return LatestVersionName
}

// GetMosVersionSuffix returns an empty string if mos version is "latest";
// otherwise returns the mos version prepended with a dash, like "-1.6".
func GetMosVersionSuffix() string {
	return moscommon.GetVersionSuffix(GetMosVersion())
}

func LooksLikeVersionNumber(s string) bool {
	return regexpVersionNumber.MatchString(s)
}

// Returns whether the build id looks like the mos was built in some distro
// environment (like, ubuntu or brew), and thus it shouldn't update itself.
func LooksLikeDistrBuildId(s string) bool {
	return ourutil.FindNamedSubmatches(regexpBuildIdDistr, s) != nil
}

func LooksLikeUbuntuBuildId(s string) bool {
	return GetUbuntuUpdateChannel(s) != ""
}

func LooksLikeBrewBuildId(s string) bool {
	return strings.HasSuffix(s, "~brew")
}

// GetUbuntuPackageName parses given build id string, and if it looks like a
// debian build id, returns either "latest" or "release". Otherwise, returns
// an empty string.
func GetUbuntuUpdateChannel(buildId string) string {
	parts := GetUbuntuBuildIDParts(buildId)
	if parts == nil {
		return ""
	}
	for _, v := range ubuntuDistrNames {
		if strings.HasPrefix(parts["distr"], v) {
			if LooksLikeVersionNumber(parts["version"]) {
				return "release"
			} else {
				return "latest"
			}
		}
	}
	return ""
}

func GetUbuntuBuildIDParts(buildId string) map[string]string {
	return ourutil.FindNamedSubmatches(regexpBuildIdDistr, buildId)
}

func GetUserAgent() string {
	return fmt.Sprintf("mos/%s %s (%s; %s)", Version, BuildId, runtime.GOOS, runtime.GOARCH)
}
