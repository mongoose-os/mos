package license

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/common/paths"
	"cesanta.com/mos/config"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/devutil"
	"cesanta.com/mos/flags"
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
	fmt.Printf("  1. Please login to %s\n", *flags.LicenseServer)
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

type licenseRequest struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id"`
	App      string `json:"app,omitempty"`
}

type licenseResponse struct {
	Text    string `json:"text"`
	Message string `json:"message"`
}

func License(ctx context.Context, devConn *dev.DevConn) error {
	var err error
	token := readToken()
	if token == "" {
		saveToken()
		token = readToken()
		if token == "" {
			return errors.New("Cannot save access token")
		}
	}
	lreq := licenseRequest{
		Type:     *flags.PID,
		DeviceID: *flags.UID,
		App:      "",
	}
	if lreq.DeviceID == "" {
		ourutil.Reportf("Connecting to the device...")
		devConn, err = devutil.CreateDevConnFromFlags(ctx)
		if err != nil {
			return errors.Annotatef(err, "error connecting to device")
		}
		ourutil.Reportf("Querying device UID...")
		rs, err := dev.CallDeviceService(ctx, devConn, "Sys.GetUID", "")
		if err != nil {
			return errors.Annotatef(err, "error querying device UID")
		}
		var res struct {
			PID string `json:"pid"`
			UID string `json:"uid"`
			App string `json:"app"`
		}
		if err := json.Unmarshal([]byte(rs), &res); err != nil || res.UID == "" || res.PID == "" {
			return errors.Annotatef(err, "invalid device reply %s", res)
		}
		ourutil.Reportf("PID: %s UID: %s", res.PID, res.UID)
		lreq.Type = res.PID
		lreq.DeviceID = res.UID
		lreq.App = res.App
	}

	ourutil.Reportf("Requesting license from %s...", *flags.LicenseServer)

	postData, _ := json.Marshal(&lreq)
	url := fmt.Sprintf("%s/api/v1/license", *flags.LicenseServer)
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postData))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	resp, err := client.Do(req)
	if err != nil {
		return errors.Annotatef(err, "HTTP request failed %s", url)
	}
	defer resp.Body.Close()
	rs, _ := ioutil.ReadAll(resp.Body)
	var lresp licenseResponse
	json.Unmarshal(rs, &lresp)
	if lresp.Text == "" {
		return errors.Errorf("Error obtaining license: %s", lresp.Message)
	}
	ourutil.Reportf("License: %s", lresp.Text)
	if devConn == nil {
		return nil
	}
	devConf, err := devConn.GetConfig(ctx)
	if err != nil {
		return errors.Annotatef(err, "failed to get config")
	}
	settings := map[string]string{
		"device.license": lresp.Text,
	}
	if err := config.ApplyDiff(devConf, settings); err != nil {
		return errors.Trace(err)
	}
	return config.SetAndSave(ctx, devConn, devConf)
}
