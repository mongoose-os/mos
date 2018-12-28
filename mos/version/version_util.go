package version

// version.go is generated separately in Makefile
// to avoid update during "blanket" go generate runs

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"cesanta.com/common/go/ourutil"
	moscommon "cesanta.com/mos/common"
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
	regexpBuildIdDistr  = regexp.MustCompile(
		`^(?P<version>[^+]+)\+(?P<hash>[^~]+)\~(?P<distr>.+)$`,
	)

	ubuntuDistrNames = []string{"xenial", "bionic", "cosmic"}
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
	matches := ourutil.FindNamedSubmatches(regexpBuildIdDistr, buildId)
	if matches != nil {
		for _, v := range ubuntuDistrNames {
			if strings.HasPrefix(matches["distr"], v) {
				if LooksLikeVersionNumber(matches["version"]) {
					return "release"
				} else {
					return "latest"
				}
			}
		}
	}
	return ""
}

func GetUserAgent() string {
	return fmt.Sprintf("mos/%s %s (%s; %s)", Version, BuildId, runtime.GOOS, runtime.GOARCH)
}
