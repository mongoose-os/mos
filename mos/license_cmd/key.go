package license

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/common/paths"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

type licenseServerAccess struct {
	Server string `json:"server,omitempty"`
	Key    string `json:"key,omitempty"`
}

type authFile struct {
	LicenseServerAccess []*licenseServerAccess `json:"license_server_access,omitempty"`
}

func readKey(server string) string {
	var auth authFile
	data, err := ioutil.ReadFile(paths.AuthFilepath)
	if err == nil {
		json.Unmarshal(data, &auth)
	}
	for _, s := range auth.LicenseServerAccess {
		if s.Server == server {
			return s.Key
		}
	}
	return ""
}

func promptKey(server string) {
	fmt.Printf(`
License server key not found.

1. Log in to %s
2. Click 'Key' in the top menu and copy the access key
3. Run "mos license-save-key ACCESS_KEY"
4. Re-run "mos license"
`+"\n", server)
}

func saveKey(server, key string) error {
	var auth authFile
	data, err := ioutil.ReadFile(paths.AuthFilepath)
	if err == nil {
		json.Unmarshal(data, &auth)
	}
	updated := false
	for _, s := range auth.LicenseServerAccess {
		if s.Server == server {
			s.Key = key
			updated = true
		}
	}
	if !updated {
		auth.LicenseServerAccess = append(auth.LicenseServerAccess, &licenseServerAccess{
			Server: server,
			Key:    key,
		})
	}
	data, _ = json.MarshalIndent(auth, "", "  ")
	if err = ioutil.WriteFile(paths.AuthFilepath, data, 0600); err == nil {
		ourutil.Reportf("Saved key for %s to %s", server, paths.AuthFilepath)
	}
	return err
}

func SaveKey(ctx context.Context, devConn dev.DevConn) error {
	key := *flags.LicenseServerKey
	if key == "" && len(flag.Args()) == 2 {
		key = flag.Args()[1]
	} else {
		return errors.Errorf("key is required %d", len(flag.Args()))
	}
	return saveKey(*flags.LicenseServer, key)
}
