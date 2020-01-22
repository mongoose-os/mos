//
// Copyright (c) 2014-2019 Cesanta Software Limited
// All rights reserved
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//go:generate go-bindata -pkg gcp -nocompress -modtime 1 -mode 420 data/

package gcp

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/juju/errors"
	"github.com/mongoose-os/mos/cli/atca"
	"github.com/mongoose-os/mos/cli/config"
	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/flags"
	"github.com/mongoose-os/mos/cli/fs"
	"github.com/mongoose-os/mos/cli/ourutil"
	"github.com/mongoose-os/mos/cli/x509utils"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudiot/v1"
)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GCPIoTSetup(ctx context.Context, devConn dev.DevConn) error {
	if *flags.GCPProject == "" || *flags.GCPRegion == "" || *flags.GCPRegistry == "" {
		return errors.Errorf("Please set --gcp-project, --gcp-region and --gcp-registry")
	}

	httpClient, err := google.DefaultClient(ctx, cloudiot.CloudPlatformScope)
	if err != nil {
		return errors.Annotatef(err, "failed to create GCP HTTP client")
	}
	iotAPIClient, err := cloudiot.New(httpClient)
	if err != nil {
		return errors.Annotatef(err, "failed to create GCP client")
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
	pubKeyFormat := ""
	switch certType {
	case x509utils.CertTypeRSA:
		pubKeyFormat = "RSA_X509_PEM"
	case x509utils.CertTypeECDSA:
		pubKeyFormat = "ES256_PEM"
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
		ctx, certType, *flags.GCPCertFile, *flags.GCPKeyFile, certTmpl, useATCA, devConn, devConf, devInfo)

	var pubKeyPEMBytes []byte
	switch certType {
	case x509utils.CertTypeRSA:
		pubKeyFileName := fmt.Sprintf("gcp-%s.crt.pem", ourutil.FirstN(certCN, 16))
		_, err = x509utils.WriteAndUploadFile(ctx, "certificate", certPEMBytes,
			*flags.GCPCertFile, pubKeyFileName, nil)
		if err != nil {
			return errors.Trace(err)
		}
	case x509utils.CertTypeECDSA:
		pubKeyDERBytes, _ := x509.MarshalPKIXPublicKey(keySigner.Public())
		pubKeyPEMBuf := bytes.NewBuffer(nil)
		pem.Encode(pubKeyPEMBuf, &pem.Block{Type: "PUBLIC KEY", Bytes: pubKeyDERBytes})
		pubKeyFileName := fmt.Sprintf("gcp-%s.pub.pem", ourutil.FirstN(certCN, 16))
		pubKeyPEMBytes = pubKeyPEMBuf.Bytes()
		_, err = x509utils.WriteAndUploadFile(ctx, "public key", pubKeyPEMBytes,
			*flags.GCPCertFile, pubKeyFileName, nil)
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
			*flags.GCPKeyFile, keyFileName, devConn)
		if err != nil {
			return errors.Trace(err)
		}
	} else if useATCA {
		keyDevFileName = fmt.Sprintf("%s%d", atca.KeyFilePrefix, x509utils.ATCASlot)
	} else {
		return errors.Errorf("BUG: no private key data!")
	}

	ourutil.Reportf("Creating the device...")
	parent := fmt.Sprintf("projects/%s/locations/%s/registries/%s",
		*flags.GCPProject, *flags.GCPRegion, *flags.GCPRegistry)
	device := cloudiot.Device{
		Id: devID,
		Credentials: []*cloudiot.DeviceCredential{
			{
				PublicKey: &cloudiot.PublicKeyCredential{
					Format: pubKeyFormat,
					Key:    string(pubKeyPEMBytes),
				},
			},
		},
	}
	resp, err := iotAPIClient.Projects.Locations.Registries.Devices.Create(parent, &device).Do()
	if err != nil {
		glog.Errorf("Error creating device: %s %#v", err, resp)
		ourutil.Reportf("Trying to delete the device...")
		dev := fmt.Sprintf("%s/devices/%s", parent, devID)
		_, err1 := iotAPIClient.Projects.Locations.Registries.Devices.Delete(dev).Do()
		if err1 != nil {
			return errors.Annotatef(err, "failed to re-create device")
		}
		ourutil.Reportf("Retrying device creation...")
		resp, err = iotAPIClient.Projects.Locations.Registries.Devices.Create(parent, &device).Do()
		if err != nil {
			return errors.Annotatef(err, "failed to create device")
		}
	}

	caCertFile := "data/gcp-lts-ca.pem"
	caCertData := MustAsset(caCertFile)
	ourutil.Reportf("Uploading CA certificate...")
	err = fs.PutData(ctx, devConn, bytes.NewBuffer(caCertData), filepath.Base(caCertFile))
	if err != nil {
		return errors.Annotatef(err, "failed to upload %s", filepath.Base(caCertFile))
	}

	newConf := map[string]string{
		"gcp.enable":   "true",
		"gcp.project":  *flags.GCPProject,
		"gcp.region":   *flags.GCPRegion,
		"gcp.registry": *flags.GCPRegistry,
		"gcp.device":   devID,
		"gcp.key":      keyDevFileName,
		"gcp.server":   "mqtt.2030.ltsapis.goog:8883",
	}
	// GCP tokens require valid wall time, enable SNTP if available.
	if _, err := devConf.Get("sntp.enable"); err == nil {
		newConf["sntp.enable"] = "true"
	}
	if _, err := devConf.Get("rpc.mqtt.enable"); err == nil {
		// GCP does not support bi-di MQTT comms, RPC won't work.
		// Turn off if it's present, don't fail if it isn't.
		newConf["rpc.mqtt.enable"] = "false"
	}

	if _, err := devConf.Get("gcp.ca_cert"); err == nil {
		// Modern GCP library.
		newConf["gcp.ca_cert"] = filepath.Base(caCertFile)
	} else {
		// Legacy: need to enable MQTT separately.
		newConf["mqtt.ssl_ca_cert"] = filepath.Base(caCertFile)
		newConf["mqtt.enable"] = "true"
	}
	if err := config.ApplyDiff(devConf, newConf); err != nil {
		return errors.Trace(err)
	}
	return config.SetAndSave(ctx, devConn, devConf)
}
