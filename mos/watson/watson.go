package watson

import (
	"context"
	"fmt"
	"strings"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/config"
	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

const (
	quickStartOrgID = "quickstart"
	mosDeviceType   = "mos"
)

var (
	watsonAPIKey      = ""
	watsonAuthToken   = ""
	watsonOrgID       = ""
	watsonDeviceType  = ""
	watsonDeviceID    = ""
	watsonIoTHostName = ""
)

func init() {
	flag.StringVar(&watsonAPIKey, "watson-api-key", "", "IBM cloud API key")
	flag.StringVar(&watsonAuthToken, "watson-auth-token", "", "IBM cloud API auth token")
	flag.StringVar(&watsonOrgID, "watson-org-id", quickStartOrgID, "IBM cloud organization ID")
	flag.StringVar(&watsonDeviceType, "watson-device-type", mosDeviceType, "IBM cloud device type")
	flag.StringVar(&watsonDeviceID, "watson-device-id", "", "IBM cloud device ID")
	flag.StringVar(&watsonIoTHostName, "watson-iot-host-name", "", "IBM cloud host name")
}

func getOrgID() string {
	if watsonOrgID != quickStartOrgID {
		return watsonOrgID
	}
	if watsonAPIKey != "" {
		parts := strings.Split(watsonAPIKey, "-")
		if len(parts) == 3 {
			return parts[1]
		}
	}
	return watsonOrgID
}

func WatsonIoTSetup(ctx context.Context, devConn *dev.DevConn) error {
	var err error
	orgID := getOrgID()
	if orgID != quickStartOrgID && watsonAuthToken == "" {
		return errors.Errorf("Org ID is provided but auth key is not set")
	}

	ourutil.Reportf("Connecting to the device...")
	devInfo, err := devConn.GetInfo(ctx)
	if err != nil {
		return errors.Annotatef(err, "failed to connect to device")
	}
	devArch, devMAC := *devInfo.Arch, *devInfo.Mac
	ourutil.Reportf("  %s %s running %s", devArch, devMAC, *devInfo.App)
	devConf, err := devConn.GetConfig(ctx)
	if err != nil {
		return errors.Annotatef(err, "failed to get config")
	}
	devID := watsonDeviceID
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
	authToken := ""
	if watsonOrgID != quickStartOrgID {
		// TODO(rojer): create device and device type if necessary.
	}

	hostName := watsonIoTHostName
	if hostName == "" {
		hostName = fmt.Sprintf("%s.messaging.internetofthings.ibmcloud.com", orgID)
	}

	newConf := map[string]string{
		"device.id":        devID,
		"watson.enable":    "true",
		"watson.host_name": hostName,
		"watson.client_id": fmt.Sprintf("d:%s:%s:%s", orgID, watsonDeviceType, devID),
	}
	if authToken != "" {
		newConf["watson.token"] = authToken
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
