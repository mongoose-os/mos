package azure

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/atca"
	"cesanta.com/mos/config"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/x509utils"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

var (
	azureDeviceID         = ""
	azureIoTHubName       = ""
	azureIoTHubHostName   = ""
	azureIoTAuthMethod    = ""
	azureIoTDeviceStatus  = ""
	azureIoTResourceGroup = ""
	azureIoTSkipCLICheck  = false
	azureCertFile         = ""
	azureKeyFile          = ""
)

func init() {
	flag.StringVar(&azureDeviceID, "azure-device-id", "", "Azure IoT device ID. If not set, taken from the device itself.")
	flag.StringVar(&azureIoTHubName, "azure-hub-name", "", "Azure IoT hub name")
	flag.StringVar(&azureIoTHubHostName, "azure-hub-host-name", "", "Azure IoT hub host name")
	flag.StringVar(&azureIoTAuthMethod, "azure-auth-method", "x509_thumbprint",
		"Azure IoT Device authentication method: x509_thumbprint, x509_ca, shared_private_key")
	flag.StringVar(&azureIoTDeviceStatus, "azure-device-status", "enabled", "Azure IoT Device status upon creation")
	flag.StringVar(&azureIoTResourceGroup, "azure-resource-group", "", "Azure resource group")
	flag.BoolVar(&azureIoTSkipCLICheck, "azure-skip-cli-check", false, "Skip Azure CLI check, assume it's ok")
	flag.StringVar(&azureCertFile, "azure-cert-file", "", "Certificate/public key file")
	flag.StringVar(&azureKeyFile, "azure-key-file", "", "Private key file")
}

type azureIoTHubInfo struct {
	Name       string `json:"name"`
	Properties struct {
		HostName string `json:"hostName"`
	} `json:"properties"`
}

func AzureIoTSetup(ctx context.Context, devConn dev.DevConn) error {
	// Perform Azure CLI sanity checks
	if !azureIoTSkipCLICheck {
		// Make sure that Azure CLI is installed and logged in.
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, "az"); err != nil {
			return errors.Annotatef(err, "Failed to run Azure CLI utility. Make sure it is installed - https://docs.microsoft.com/en-us/cli/azure/install-azure-cli")
		}
		// Check that IoT extension is installed
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, "az", "extension", "show", "--name", "azure-cli-iot-ext"); err != nil {
			ourutil.Reportf("Installing azure-cli-iot extension...")
			if err := ourutil.RunCmd(ourutil.CmdOutOnError, "az", "extension", "add", "--name", "azure-cli-iot-ext", "--yes"); err != nil {
				return errors.Annotatef(err, "azure-cli-iot-ext was not found and could not be installed. Please do it manually")
			}
		}
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, "az", "account", "show"); err != nil {
			if err := ourutil.RunCmd(ourutil.CmdOutAlways, "az", "login"); err != nil {
				return errors.Annotatef(err, "Azure CLI is not logged in and 'az login' failed. Please login manually.")
			}
		}
	}
	if azureIoTHubName == "" {
		output, err := ourutil.GetCommandOutput("az", "iot", "hub", "list")
		if err != nil {
			return errors.Annotatef(err, "--azure-hub-name not specified and list command failed")
		}
		var hubsInfo []azureIoTHubInfo
		if err = json.Unmarshal([]byte(output), &hubsInfo); err != nil {
			return errors.Annotatef(err, "invalist az iot hub list output format")
		}
		switch len(hubsInfo) {
		case 0:
			return errors.Errorf("there are no Azure IoT hubs present, please create one " +
				"as described here: https://mongoose-os.com/docs/cloud/azure.md")
		case 1:
			azureIoTHubName = hubsInfo[0].Name
			azureIoTHubHostName = hubsInfo[0].Properties.HostName
		default:
			return errors.Errorf("there is more than one Azure IoT hub, please specify --azure-hub-name")
		}
	} else if azureIoTHubHostName == "" {
		// Check that the specified hub exists.
		output, err := ourutil.GetCommandOutput("az", "iot", "hub", "show", "--name", azureIoTHubName)
		if err != nil {
			return errors.Errorf("Azure IoT Hub %q does not exist, please create it "+
				"as described here: https://mongoose-os.com/docs/cloud/azure.md", azureIoTHubName)
		}
		var hubInfo azureIoTHubInfo
		if err = json.Unmarshal([]byte(output), &hubInfo); err != nil {
			return errors.Annotatef(err, "invalist az iot hub list output format")
		}
		azureIoTHubHostName = hubInfo.Properties.HostName
	}

	ourutil.Reportf("Using IoT hub %s (%s)", azureIoTHubName, azureIoTHubHostName)

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
	devID := azureDeviceID
	if devID == "" {
		devID, err = devConf.Get("device.id")
		if err != nil {
			return errors.Annotatef(err, "failed to get device.id from config")
		}
	}
	_, err = devConf.Get("azure")
	if err != nil {
		return errors.Annotatef(err, "failed to get current Azure config. Make sure firmware is built "+
			"with the Azure support library (https://github.com/mongoose-os-libs/azure)")
	}

	certType, useATCA, err := x509utils.PickCertType(devInfo)
	if err != nil {
		return errors.Trace(err)
	}

	createArgs := []string{
		"az", "iot", "hub", "device-identity", "create",
		"--hub-name", azureIoTHubName,
		"--device-id", devID,
		"--auth-method", azureIoTAuthMethod,
		"--status", azureIoTDeviceStatus,
	}
	if azureIoTResourceGroup != "" {
		createArgs = append(createArgs, "--resource-group", azureIoTResourceGroup)
	}

	certCN := x509utils.CertCN
	if certCN == "" {
		certCN = devID
	}
	var certPEMBytes, keyPEMBytes []byte
	var certFPHex string
	switch azureIoTAuthMethod {
	case "x509_thumbprint":
		var certTmpl *x509.Certificate
		if azureIoTAuthMethod == "x509_thumbprint" {
			certTmpl = &x509.Certificate{}
			certTmpl.Subject.CommonName = certCN
		}
		var certDERBytes []byte
		certDERBytes, certPEMBytes, _, _, keyPEMBytes, err = x509utils.LoadOrGenerateCertAndKey(
			ctx, certType, azureCertFile, azureKeyFile, certTmpl, useATCA, devConn, devConf, devInfo)
		if err != nil {
			return errors.Annotatef(err, "failed to generate certificate")
		}
		certFP := sha1.Sum(certDERBytes)
		certFPHex = strings.ToUpper(hex.EncodeToString(certFP[:]))
		createArgs = append(createArgs, "--primary-thumbprint", certFPHex)
	case "x509_ca":
	case "shared_private_key":
		// TODO(rojer): Implement.
		return errors.NotImplementedf("shared_private_key auth method")
	}
	ourutil.Reportf("  SHA1 FP : %s", certFPHex)

	ourutil.Reportf("Creating the device...")
	if err := ourutil.RunCmd(ourutil.CmdOutOnError, createArgs...); err != nil {
		// Most likely device already exists, try deleting.
		ourutil.Reportf("Trying to delete the device...")
		deleteArgs := []string{
			"az", "iot", "hub", "device-identity", "delete",
			"--hub-name", azureIoTHubName,
			"--device-id", devID,
		}
		if azureIoTResourceGroup != "" {
			createArgs = append(createArgs, "--resource-group", azureIoTResourceGroup)
		}
		if ourutil.RunCmd(ourutil.CmdOutOnError, deleteArgs...) != nil {
			return errors.Annotatef(err, "failed to re-create device")
		}
		// Try again
		ourutil.Reportf("Retrying device creation...")
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, createArgs...); err != nil {
			return errors.Annotatef(err, "failed to create device")
		}
	}

	newConf := map[string]string{
		"device.id":    devID,
		"azure.enable": "true",
	}

	switch azureIoTAuthMethod {
	case "x509_ca":
		fallthrough
	case "x509_thumbprint":
		fileNameSuffix := ourutil.FirstN(certCN, 16)
		newConf["azure.cs"] = ""
		newConf["azure.host_name"] = azureIoTHubHostName
		newConf["azure.device_id"] = devID
		if certPEMBytes != nil {
			certFileName := fmt.Sprintf("azure-%s.crt.pem", fileNameSuffix)
			certDevFileName, err := x509utils.WriteAndUploadFile(ctx, "certificate", certPEMBytes,
				azureCertFile, certFileName, devConn)
			if err != nil {
				return errors.Trace(err)
			}
			newConf["azure.cert"] = certDevFileName
		}
		keyDevFileName := ""
		if keyPEMBytes != nil {
			keyFileName := fmt.Sprintf("azure-%s.key.pem", fileNameSuffix)
			keyDevFileName, err = x509utils.WriteAndUploadFile(ctx, "key", keyPEMBytes,
				azureKeyFile, keyFileName, devConn)
			if err != nil {
				return errors.Trace(err)
			}
		} else if useATCA {
			keyDevFileName = fmt.Sprintf("%s%d", atca.KeyFilePrefix, x509utils.ATCASlot)
		} else {
			return errors.Errorf("BUG: no private key data!")
		}
		newConf["azure.key"] = keyDevFileName

	case "shared_private_key":
		// TODO(rojer): Implement.
		return errors.NotImplementedf("shared_private_key auth method")
	}

	// Azure does not support bi-di MQTT comms, RPC won't work.
	// Turn off if it's present, don't fail if it isn't.
	devConf.Set("rpc.mqtt.enable", "false")

	if err := config.ApplyDiff(devConf, newConf); err != nil {
		return errors.Trace(err)
	}

	return config.SetAndSave(ctx, devConn, devConf)
}
