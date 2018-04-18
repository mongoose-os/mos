//go:generate go-bindata -pkg gcp -nocompress -modtime 1 -mode 420 data/

package gcp

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/atca"
	"cesanta.com/mos/config"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/fs"
	"cesanta.com/mos/x509utils"
	"github.com/cesanta/errors"
)

var (
	gcpProject  = ""
	gcpRegion   = ""
	gcpRegistry = ""
	gcpCertFile = ""
	gcpKeyFile  = ""
)

func init() {
	flag.StringVar(&gcpProject, "gcp-project", "", "Google IoT project ID")
	flag.StringVar(&gcpRegion, "gcp-region", "", "Google IoT region")
	flag.StringVar(&gcpRegistry, "gcp-registry", "", "Google IoT device registry")
	flag.StringVar(&gcpCertFile, "gcp-cert-file", "", "Certificate/public key file")
	flag.StringVar(&gcpKeyFile, "gcp-key-file", "", "Private key file")
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GCPIoTSetup(ctx context.Context, devConn *dev.DevConn) error {
	if gcpProject == "" || gcpRegion == "" || gcpRegistry == "" {
		return errors.Errorf("Please set --gcp-project, --gcp-region and --gcp-registry")
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
		return errors.Annotatef(err, "failed to connect to get device config")
	}
	devID, err := devConf.Get("device.id")
	if err != nil {
		return errors.Annotatef(err, "failed to get device ID")
	}
	_, err = devConf.Get("gcp")
	if err != nil {
		return errors.Annotatef(err, "failed to get GCP config. Make sure the firmware supports GCP")
	}

	certType, useATCA, err := x509utils.PickCertType(devInfo)
	if err != nil {
		return errors.Trace(err)
	}
	jwsType := ""
	switch certType {
	case x509utils.CertTypeRSA:
		jwsType = "rs256"
	case x509utils.CertTypeECDSA:
		jwsType = "es256"
	default:
		return errors.Errorf("unsupported certy type %s", certType)
	}

	certCN := x509utils.CertCN
	if certCN == "" {
		certCN = devID
	}
	var certTmpl *x509.Certificate
	certTmpl = &x509.Certificate{}
	certTmpl.Subject.CommonName = certCN
	_, certPEMBytes, keySigner, _, keyPEMBytes, err := x509utils.LoadOrGenerateCertAndKey(
		ctx, certType, gcpCertFile, gcpKeyFile, certTmpl, useATCA, devConn, devConf, devInfo)

	pubKeyFileName := ""
	switch certType {
	case x509utils.CertTypeRSA:
		pubKeyFileName = fmt.Sprintf("gcp-%s.crt.pem", ourutil.FirstN(certCN, 16))
		_, err = x509utils.WriteAndUploadFile(ctx, "certificate", certPEMBytes,
			gcpCertFile, pubKeyFileName, nil)
		if err != nil {
			return errors.Trace(err)
		}
	case x509utils.CertTypeECDSA:
		pubKeyDERBytes, _ := x509.MarshalPKIXPublicKey(keySigner.Public())
		pubKeyPEMBuf := bytes.NewBuffer(nil)
		pem.Encode(pubKeyPEMBuf, &pem.Block{Type: "PUBLIC KEY", Bytes: pubKeyDERBytes})
		pubKeyFileName = fmt.Sprintf("gcp-%s.pub.pem", ourutil.FirstN(certCN, 16))
		_, err = x509utils.WriteAndUploadFile(ctx, "public key", pubKeyPEMBuf.Bytes(),
			gcpCertFile, pubKeyFileName, nil)
		if err != nil {
			return errors.Trace(err)
		}
	default:
		return errors.Errorf("unsupported certy type %s", certType)
	}
	keyDevFileName := ""
	if keyPEMBytes != nil {
		keyFileName := fmt.Sprintf("gcp-%s.key.pem", ourutil.FirstN(certCN, 16))
		keyDevFileName, err = x509utils.WriteAndUploadFile(ctx, "key", keyPEMBytes,
			gcpKeyFile, keyFileName, devConn)
		if err != nil {
			return errors.Trace(err)
		}
	} else if useATCA {
		keyDevFileName = fmt.Sprintf("%s%d", atca.KeyFilePrefix, x509utils.ATCASlot)
	} else {
		return errors.Errorf("BUG: no private key data!")
	}

	createArgs := []string{
		"gcloud", "iot", "devices", "create", devID,
		"--project", gcpProject, "--region", gcpRegion, "--registry", gcpRegistry,
		"--public-key", fmt.Sprintf("path=%s,type=%s", pubKeyFileName, jwsType),
	}
	ourutil.Reportf("Creating the device...")
	if err := ourutil.RunCmd(ourutil.CmdOutOnError, createArgs...); err != nil {
		ourutil.Reportf("Trying to delete the device...")
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, "gcloud", "iot", "devices", "delete", devID,
			"--project", gcpProject, "--region", gcpRegion, "--registry", gcpRegistry, "--quiet"); err != nil {
			return errors.Annotatef(err, "failed to re-create device")
		}
		ourutil.Reportf("Retrying device creation...")
		if err := ourutil.RunCmd(ourutil.CmdOutOnError, createArgs...); err != nil {
			return errors.Annotatef(err, "failed to create device")
		}
	}

	// ca.pem has both roots in it, so, for platforms other than CC32XX, we can just use that.
	// CC32XX do not support cert bundles and will always require specific CA cert.
	// http://e2e.ti.com/support/wireless_connectivity/simplelink_wifi_cc31xx_cc32xx/f/968/t/634431
	caCertFile := "ca.pem"
	uploadCACert := false
	if strings.HasPrefix(strings.ToLower(*devInfo.Arch), "cc320") {
		caCertFile = "data/ca-globalsign.crt.pem"
		uploadCACert = true
	}

	if uploadCACert {
		caCertData := MustAsset(caCertFile)
		ourutil.Reportf("Uploading CA certificate...")
		err = fs.PutData(ctx, devConn, bytes.NewBuffer(caCertData), filepath.Base(caCertFile))
		if err != nil {
			return errors.Annotatef(err, "failed to upload %s", filepath.Base(caCertFile))
		}
	}

	// GCP does not support bi-di MQTT comms, RPC won't work.
	// Turn off if it's present, don't fail if it isn't.
	devConf.Set("rpc.mqtt.enable", "false")

	newConf := map[string]string{
		"sntp.enable":      "true",
		"mqtt.enable":      "true",
		"mqtt.server":      "mqtt.googleapis.com:8883",
		"mqtt.ssl_ca_cert": filepath.Base(caCertFile),
		"gcp.enable":       "true",
		"gcp.project":      gcpProject,
		"gcp.region":       gcpRegion,
		"gcp.registry":     gcpRegistry,
		"gcp.device":       devID,
		"gcp.key":          keyDevFileName,
	}
	if err := config.ApplyDiff(devConf, newConf); err != nil {
		return errors.Trace(err)
	}
	return config.SetAndSave(ctx, devConn, devConf)
}
