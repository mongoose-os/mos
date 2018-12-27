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

type LicenseServerAccess struct {
	Server string `json:"server,omitempty"`
	Token  string `json:"token,omitempty"`
}

type AuthFile struct {
	LicenseServerAccess []*LicenseServerAccess `json:"license_server_access,omitempty"`
}

func readToken(server string) string {
	var auth AuthFile
	data, err := ioutil.ReadFile(paths.AuthFilepath)
	if err == nil {
		json.Unmarshal(data, &auth)
	}
	for _, s := range auth.LicenseServerAccess {
		if s.Server == server {
			return s.Token
		}
	}
	return ""
}

func promptToken(server string) string {
	fmt.Printf("  1. Please login to %s\n", server)
	fmt.Println("  2. Click on 'Key' in the top menu")
	fmt.Println("  3. Copy the access key to the clipboard")
	fmt.Println("  4. Paste the access key below and press enter")
	fmt.Print("Access key: ")
	scanner := bufio.NewScanner(bufio.NewReader(os.Stdin))
	scanner.Scan()
	return scanner.Text()
}

func saveToken(server, token string) {
	var auth AuthFile
	data, err := ioutil.ReadFile(paths.AuthFilepath)
	if err == nil {
		json.Unmarshal(data, &auth)
	}
	updated := false
	for _, s := range auth.LicenseServerAccess {
		if s.Server == server {
			s.Token = token
			updated = true
		}
	}
	if !updated {
		auth.LicenseServerAccess = append(auth.LicenseServerAccess, &LicenseServerAccess{
			Server: server,
			Token:  token,
		})
	}
	data, _ = json.MarshalIndent(auth, "", "  ")
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
	server := *flags.LicenseServer
	token := readToken(server)
	shouldSaveToken := false
	if token == "" {
		token = promptToken(server)
		if token == "" {
			return errors.New("Failed to obtain access token")
		}
		shouldSaveToken = true
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

	ourutil.Reportf("Requesting license from %s...", server)

	postData, _ := json.Marshal(&lreq)
	url := fmt.Sprintf("%s/api/v1/license", server)
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
	if shouldSaveToken {
		saveToken(server, token)
	}
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
