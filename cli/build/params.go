package build

// Last-minute adjustments for the manifest, typically constructed from command line
type ManifestAdjustments struct {
	Platform  string
	BuildVars map[string]string
	CDefs     map[string]string
	CFlags    []string
	CXXFlags  []string
	ExtraLibs []SWModule
}

// Note: this struct gets transmitted to the server
type BuildParams struct {
	ManifestAdjustments
	BuildTarget           string
	CustomLibLocations    map[string]string
	CustomModuleLocations map[string]string
	NoPlatformCheck       bool
	// host -> credentials, used for authentication when fetching libs.
	Credentials map[string]Credentials
}

type Credentials struct {
	User string
	Pass string
}

func (bp *BuildParams) GetCredentialsForHost(host string) *Credentials {
	if bp.Credentials == nil {
		return nil
	}
	creds, ok := bp.Credentials[host]
	if ok {
		return &creds
	}
	creds, ok = bp.Credentials[""]
	if ok {
		return &creds
	}
	return nil
}
