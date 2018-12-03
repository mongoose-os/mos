package flags

import (
	"cesanta.com/common/go/ourutil"
	flag "github.com/spf13/pflag"
)

var (
	// --arch was deprecated at 2017/08/15 and should eventually be removed.
	archOld     = flag.String("arch", "", "Deprecated, please use --platform instead")
	Board       = flag.String("board", "", "Board name.")
	BuildInfo   = flag.String("build-info", "", "")
	Checksums   = flag.StringSlice("checksums", []string{"sha1"}, "")
	Description = flag.String("description", "", "")
	Input       = flag.StringP("input", "i", "", "")
	Manifest    = flag.String("manifest", "", "")
	Name        = flag.String("name", "", "")
	Output      = flag.StringP("output", "o", "", "")
	platform    = flag.String("platform", "", "Hardware platform")
	SrcDir      = flag.String("src-dir", "", "")
	Compress    = flag.Bool("compress", false, "")
	GHToken     = flag.String("gh-token", "", "")
)

func Platform() string {
	if *platform != "" {
		return *platform
	}
	if *archOld != "" {
		ourutil.Reportf("Warning: --arch is deprecated, use --platform")
	}
	return *archOld
}
