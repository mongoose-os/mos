package license

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"cesanta.com/mos/common/paths"
	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
)

type AuthFile struct {
	LicenseServerAccessToken string `json:"license_server_access_token"`
}

func readToken() string {
	auth := AuthFile{}
	data, _ := ioutil.ReadFile(paths.AuthFilepath)
	json.Unmarshal(data, &auth)
	return auth.LicenseServerAccessToken
}

func prompt() string {
	fmt.Println("  1. Please login to https://license.mongoose-os.com")
	fmt.Println("  2. Click on 'Key' in the top menu")
	fmt.Println("  3. Copy the access key to the clipboard")
	fmt.Println("  4. Paste the access key below and press enter")
	fmt.Print("Access key: ")
	scanner := bufio.NewScanner(bufio.NewReader(os.Stdin))
	scanner.Scan()
	return scanner.Text()
}

func saveToken() {
	token := prompt()
	auth := AuthFile{LicenseServerAccessToken: token}
	data, _ := json.MarshalIndent(auth, "", "  ")
	ioutil.WriteFile(paths.AuthFilepath, data, 0600)
}

func License(ctx context.Context, devConn *dev.DevConn) error {
	token := readToken()
	if token == "" {
		saveToken()
		token = readToken()
		if token == "" {
			return errors.New("Cannot save access token")
		}
	}
	return errors.New("Are you looking at me? Not implemented yet!")
}
