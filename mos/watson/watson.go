package watson

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/config"
	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

const (
	quickStartOrgID = "quickstart"
	mosDeviceType   = "mos"
)

var (
	WatsonAPIKeyFlag            = ""
	WatsonAPIAuthTokenFlag      = ""
	watsonOrgIDFlag             = ""
	watsonDeviceTypeFlag        = ""
	watsonDeviceIDFlag          = ""
	watsonMessagingHostNameFlag = ""
	watsonAPIHostNameFlag       = ""
	watsonDeviceAuthTokenFlag   = ""
)

func init() {
	flag.StringVar(&WatsonAPIKeyFlag, "watson-api-key", "", "IBM cloud API key")
	flag.StringVar(&WatsonAPIAuthTokenFlag, "watson-api-auth-token", "", "IBM cloud API auth token")
	flag.StringVar(&watsonOrgIDFlag, "watson-org-id", quickStartOrgID, "IBM cloud organization ID")
	flag.StringVar(&watsonDeviceTypeFlag, "watson-device-type", mosDeviceType, "IBM cloud device type")
	flag.StringVar(&watsonDeviceIDFlag, "watson-device-id", "", "IBM cloud device ID")
	flag.StringVar(&watsonAPIHostNameFlag, "watson-api-host-name", "", "IBM cloud API host name")
	flag.StringVar(&watsonMessagingHostNameFlag, "watson-messaging-host-name", "", "IBM cloud host name")
	flag.StringVar(&watsonDeviceAuthTokenFlag, "watson-device-auth-token", "", "IBM cloud device auth token")
}

func getOrgID() string {
	if watsonOrgIDFlag != quickStartOrgID {
		return watsonOrgIDFlag
	}
	if WatsonAPIKeyFlag != "" {
		parts := strings.Split(WatsonAPIKeyFlag, "-")
		if len(parts) == 3 {
			return parts[1]
		}
	}
	return watsonOrgIDFlag
}

type watsonDeviceAdditionRequest struct {
	DeviceID   string           `json:"deviceId,omitempty"`
	AuthToken  string           `json:"authToken,omitempty"`
	DeviceInfo watsonDeviceInfo `json:"deviceInfo,omitempty"`
}

type watsonDeviceInfo struct {
	Description string `json:"description,omitempty"`
	DeviceClass string `json:"deviceClass,omitempty"`
}

type watsonDeviceTypeInfo struct {
	ID          string `json:"id,omitempty"`
	Description string `json:"description,omitempty"`
	ClassID     string `json:"classId,omitempty"`
}

type watsonDeviceTypeInfoList struct {
	Results []watsonDeviceTypeInfo `json:"results"`
}

func watsonAPICall(reqType, hostName, apiKey, apiToken, api string, jsonReq, jsonResp interface{}) (int, error) {
	client := &http.Client{}
	url := fmt.Sprintf("https://%s/api/v0002%s", hostName, api)
	var reqBody io.Reader
	if jsonReq != nil {
		jb, err := json.Marshal(jsonReq)
		if err != nil {
			return 0, errors.Annotatef(err, "invalid request body")
		}
		reqBody = bytes.NewBuffer(jb)
	}
	req, err := http.NewRequest(reqType, url, reqBody)
	req.SetBasicAuth(apiKey, apiToken)
	if reqType == "POST" {
		req.Header.Add("Content-Type", "application/json")
	}
	glog.V(2).Infof("%s %s %s", reqType, url, reqBody)
	resp, err := client.Do(req)
	if err != nil {
		return 0, errors.Annotatef(err, "API request failed (%s)", url)
	}
	b, _ := ioutil.ReadAll(resp.Body)
	glog.V(2).Infof("Resp: %d %s", resp.StatusCode, string(b))
	if resp.StatusCode >= 300 {
		return resp.StatusCode, errors.Errorf("API request failed (%s): %d %s", url, resp.StatusCode, string(b))
	}
	defer resp.Body.Close()
	if jsonResp != nil {
		if err := json.Unmarshal(b, jsonResp); err != nil {
			return 0, errors.Annotatef(err, "invalid response format %s", b)
		}
	}
	return resp.StatusCode, nil
}

func checkDeviceType(hostName, devType, apiKey, apiToken string) error {
	ourutil.Reportf("Checking device type %q...", devType)
	var dtl watsonDeviceTypeInfoList
	if _, err := watsonAPICall("GET", hostName, apiKey, apiToken, "/device/types", nil, &dtl); err != nil {
		return errors.Annotatef(err, "failed to get device type list")
	}
	found := false
	for _, dt := range dtl.Results {
		if dt.ID == devType {
			found = true
			break
		}
	}
	if !found {
		ourutil.Reportf("  Creating device type %q...", devType)
		dt := watsonDeviceTypeInfo{
			ID:          devType,
			Description: "Mongoose OS device type (created by mos watson-iot-setup)",
			ClassID:     "Device",
		}
		if _, err := watsonAPICall("POST", hostName, apiKey, apiToken, "/device/types", &dt, &dtl); err != nil {
			return errors.Annotatef(err, "failed to create device type %q", devType)
		}
	}
	return nil
}

func WatsonIoTSetup(ctx context.Context, devConn dev.DevConn) error {
	var err error
	orgID := getOrgID()
	if orgID != quickStartOrgID && (WatsonAPIKeyFlag == "" || WatsonAPIAuthTokenFlag == "") {
		return errors.Errorf("Org ID is provided but API key and auth token are not set")
	}

	ourutil.Reportf("Connecting to the device...")
	devInfo, err := dev.GetInfo(ctx, devConn)
	if err != nil {
		return errors.Annotatef(err, "failed to connect to device")
	}
	devArch, devMAC := *devInfo.Arch, *devInfo.Mac
	ourutil.Reportf("  %s %s running %s", devArch, devMAC, *devInfo.App)
	devConf, err := dev.GetConfig(ctx, devConn)
	if err != nil {
		return errors.Annotatef(err, "failed to get config")
	}
	devID := watsonDeviceIDFlag
	if devID == "" {
		devID, err = devConf.Get("device.id")
		if err != nil {
			return errors.Annotatef(err, "failed to get device.id from config")
		}
	}
	_, err = devConf.Get("watson")
	if err != nil {
		return errors.Annotatef(err, "failed to get current IBM Watson config. Make sure firmware is built "+
			"with the Azure support library (https://github.com/mongoose-os-libs/watson)")
	}

	ourutil.Reportf("Org ID: %s", orgID)
	ourutil.Reportf("Device ID: %s", devID)

	apiHostName := watsonAPIHostNameFlag
	if apiHostName == "" {
		apiHostName = fmt.Sprintf("%s.internetofthings.ibmcloud.com", orgID)
	}
	messagingHostName := watsonMessagingHostNameFlag
	if messagingHostName == "" {
		messagingHostName = fmt.Sprintf("%s.messaging.internetofthings.ibmcloud.com", orgID)
	}

	devType := watsonDeviceTypeFlag
	authToken := watsonDeviceAuthTokenFlag
	if orgID != quickStartOrgID {
		if err := checkDeviceType(apiHostName, devType, WatsonAPIKeyFlag, WatsonAPIAuthTokenFlag); err != nil {
			return errors.Trace(err)
		}
		kb := make([]byte, 16)
		rand.Read(kb)
		authToken = base64.URLEncoding.EncodeToString(kb)[:20]
		ourutil.Reportf("Creating device %q...", devID)
		dcr := watsonDeviceAdditionRequest{
			DeviceID:  devID,
			AuthToken: authToken,
			DeviceInfo: watsonDeviceInfo{
				Description: "Created by mos watson-iot-setup",
				DeviceClass: "Device",
			},
		}
		code, err := watsonAPICall("POST", apiHostName, WatsonAPIKeyFlag, WatsonAPIAuthTokenFlag,
			fmt.Sprintf("/device/types/%s/devices", devType), &dcr, nil)
		if err != nil && code != 409 {
			return errors.Annotatef(err, "failed to create device %q", devID)
		} else if code == 409 {
			ourutil.Reportf("  Already exists, deleting...")
			if _, err := watsonAPICall("DELETE", apiHostName, WatsonAPIKeyFlag, WatsonAPIAuthTokenFlag,
				fmt.Sprintf("/device/types/%s/devices/%s", devType, devID), nil, nil); err != nil {
				return errors.Annotatef(err, "failed to delete device %q", devID)
			}
			ourutil.Reportf("  Re-creating...")
			if _, err := watsonAPICall("POST", apiHostName, WatsonAPIKeyFlag, WatsonAPIAuthTokenFlag,
				fmt.Sprintf("/device/types/%s/devices", devType), &dcr, nil); err != nil {
				return errors.Annotatef(err, "failed to re-create the device %q", devID)
			}
		}
	}

	newConf := map[string]string{
		"device.id":        devID,
		"watson.enable":    "true",
		"watson.host_name": messagingHostName,
		"watson.client_id": fmt.Sprintf("d:%s:%s:%s", orgID, watsonDeviceTypeFlag, devID),
	}
	if authToken != "" {
		newConf["watson.token"] = authToken
	}

	// If the firmware is compiler with RPC over MQTT support, configure it to work with Watson.
	if _, err := devConf.Get("rpc.mqtt"); err == nil {
		if orgID != quickStartOrgID {
			newConf["rpc.mqtt.enable"] = "true"
			newConf["rpc.mqtt.pub_topic"] = "iot-2/evt/mgrpc-%.*s/fmt/json"
			newConf["rpc.mqtt.sub_topic"] = "iot-2/cmd/mgrpc-%.*s/fmt/json"
			newConf["rpc.mqtt.sub_wc"] = "false"
		} else {
			// Subscribing to commands is not supported on Quickstart
			// and trying to do so results in immediate disconnection.
			newConf["rpc.mqtt.enable"] = "false"
			ourutil.Reportf("Note: RPC is not supported on Quickstart service, disabling")
		}
	}

	if err = config.ApplyDiff(devConf, newConf); err != nil {
		return errors.Trace(err)
	}

	if err = config.SetAndSave(ctx, devConn, devConf); err != nil {
		return errors.Trace(err)
	}

	if orgID == quickStartOrgID {
		ourutil.Reportf("QuickStart setup complete, go to https://%s.internetofthings.ibmcloud.com/#/device/%s/sensor/ to see data", orgID, devID)
	}

	return err
}
