package build

import "time"

// Last-minute adjustments for the manifest, typically constructed from command line
type ManifestAdjustments struct {
	Platform  string
	BuildVars map[string]string
	CDefs     map[string]string
	CFlags    []string
	CXXFlags  []string
	ExtraLibs []SWModule

	// Libs and module version requirements.
	DepsVersions       *DepsManifest
	StrictDepsVersions bool
}

// Note: this struct gets transmitted to the server
type BuildParams struct {
	ManifestAdjustments
	Clean                 bool
	DryRun                bool
	Verbose               bool
	BuildTarget           string
	CustomLibLocations    map[string]string
	CustomModuleLocations map[string]string
	LibsUpdateInterval    time.Duration
	NoPlatformCheck       bool
	SaveBuildStat         bool
	PreferPrebuiltLibs    bool

	// Host -> credentials, used for authentication when fetching libs.
	Credentials map[string]Credentials
}

type Credentials struct {
	User string
	Pass string
}

func GetCredentialsForHost(credsMap map[string]Credentials, host string) *Credentials {
	if len(credsMap) == 0 {
		return nil
	}
	creds, ok := credsMap[host]
	if ok {
		return &creds
	}
	creds, ok = credsMap[""]
	if ok {
		return &creds
	}
	return nil
}

func (bp *BuildParams) GetCredentialsForHost(host string) *Credentials {
	return GetCredentialsForHost(bp.Credentials, host)
}
