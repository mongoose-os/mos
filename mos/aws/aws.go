//go:generate go-bindata -pkg aws -nocompress -modtime 1 -mode 420 data/

package aws

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/atca"
	"cesanta.com/mos/config"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/fs"
	"cesanta.com/mos/x509utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iot"
	"github.com/cesanta/errors"
	"github.com/go-ini/ini"
	"github.com/golang/glog"
	flag "github.com/spf13/pflag"
)

const (
	rsaCACert = "data/starfield.crt.pem"

	awsIoTPolicyNone        = "-"
	AWSIoTPolicyMOS         = "mos-default"
	awsIoTPolicyMOSDocument = `{"Statement": [{"Effect": "Allow", "Action": "iot:*", "Resource": "*"}], "Version": "2012-10-17"}`
)

var (
	awsGGEnable   = false
	awsMQTTServer = ""
	AWSRegion     = ""
	AWSIoTPolicy  = ""
	awsIoTThing   = ""
	IsUI          = false // XXX: Hack!
	awsCertFile   = ""
	awsKeyFile    = ""
)

func init() {
	flag.BoolVar(&awsGGEnable, "aws-enable-greengrass", false, "Enable AWS Greengrass support")
	flag.StringVar(&awsMQTTServer, "aws-mqtt-server", "", "If not specified, calls DescribeEndpoint to get it from AWS")
	flag.StringVar(&AWSRegion, "aws-region", "", "AWS region to use. If not specified, uses the default")
	flag.StringVar(&AWSIoTPolicy, "aws-iot-policy", AWSIoTPolicyMOS, "Attach this policy to the generated certificate")
	flag.StringVar(&awsIoTThing, "aws-iot-thing", "",
		"Attach the generated certificate to this thing. "+
			"By default uses device ID. Set to '-' to not attach certificate to any thing.")
	flag.StringVar(&awsCertFile, "aws-cert-file", "", "Certificate/public key file")
	flag.StringVar(&awsKeyFile, "aws-key-file", "", "Private key file")

	// Make sure we have our certs compiled in.
	MustAsset(rsaCACert)
}

func getSvc(region, keyID, key string) (*iot.IoT, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, errors.Trace(err)
	}
	cfg := defaults.Get().Config

	if region == "" {
		output, err := ourutil.GetCommandOutput("aws", "configure", "get", "region")
		if err != nil {
			if cfg.Region == nil || *cfg.Region == "" {
				ourutil.Reportf("Failed to get default AWS region, please specify --aws-region")
				return nil, errors.New("AWS region not specified")
			} else {
				region = *cfg.Region
			}
		} else {
			region = strings.TrimSpace(output)
		}
	}

	ourutil.Reportf("AWS region: %s", region)
	cfg.Region = aws.String(region)

	creds, err := GetCredentials(keyID, key)
	if err != nil {
		// In UI mode, UI credentials are acquired in a different way.
		if IsUI {
			return nil, errors.Trace(err)
		}
		creds, err = askForCreds()
		if err != nil {
			return nil, errors.Annotatef(err, "bad AWS credentials")
		}
	}
	cfg.Credentials = creds
	return iot.New(sess, cfg), nil
}

func GetCredentials(keyID, key string) (*credentials.Credentials, error) {
	if keyID != "" && key != "" {
		return credentials.NewStaticCredentials(keyID, key, ""), nil
	}
	// Try environment first, fall back to shared.
	creds := credentials.NewEnvCredentials()
	_, err := creds.Get()
	if err != nil {
		creds = credentials.NewSharedCredentials("", "")
		_, err = creds.Get()
	}
	return creds, err
}

func GetRegions() []string {
	resolver := endpoints.DefaultResolver()
	partitions := resolver.(endpoints.EnumPartitions).Partitions()
	endpoints := partitions[0].Services()["iot"]
	endpointsCN := partitions[1].Services()["iot"]
	var regions []string
	for k := range endpoints.Endpoints() {
		regions = append(regions, k)
	}
	for k := range endpointsCN.Endpoints() {
		regions = append(regions, k)
	}
	sort.Strings(regions)
	return regions
}

func GetIoTThings(region, keyID, key string) (string, error) {
	iotSvc, err := getSvc(region, keyID, key)
	if err != nil {
		return "", errors.Trace(err)
	}
	things, err := iotSvc.ListThings(&iot.ListThingsInput{})
	if err != nil {
		return "", errors.Trace(err)
	}
	return things.String(), nil
}

func GetAWSIoTPolicyNames(region, keyID, key string) ([]string, error) {
	iotSvc, err := getSvc(region, keyID, key)
	if err != nil {
		return nil, errors.Trace(err)
	}
	lpr, err := iotSvc.ListPolicies(&iot.ListPoliciesInput{})
	if err != nil {
		return nil, errors.Trace(err)
	}
	var policies []string
	for _, p := range lpr.Policies {
		policies = append(policies, *p.PolicyName)
	}
	sort.Strings(policies)
	return policies, nil
}

func genCert(ctx context.Context, certType x509utils.CertType, useATCA bool, iotSvc *iot.IoT, devConn dev.DevConn, devConf *dev.DevConf, devInfo *dev.GetInfoResult, cn, thingName, region, policy, keyID, key string) ([]byte, []byte, error) {
	var err error

	if policy == "" {
		policies, err := GetAWSIoTPolicyNames(region, keyID, key)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
		return nil, nil, errors.Errorf("--aws-iot-policy is not set. Please choose a security policy to attach "+
			"to the new certificate. --aws-iot-policy=%s will create a default permissive policy; or set --aws-iot-policy=%s to not attach any.\nExisting policies: %s",
			AWSIoTPolicyMOS, awsIoTPolicyNone, strings.Join(policies, " "))
	}

	keySigner, _, keyPEMBytes, err := x509utils.GeneratePrivateKey(ctx, certType, useATCA, devConn, devConf, devInfo)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to generate private key")
	}

	ourutil.Reportf("Generating certificate request, CN: %s", cn)
	csrTmpl := &x509.CertificateRequest{}
	csrTmpl.Subject.CommonName = cn
	csrData, err := x509.CreateCertificateRequest(rand.Reader, csrTmpl, keySigner)
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to generate CSR")
	}
	pemBuf := bytes.NewBuffer(nil)
	pem.Encode(pemBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrData})

	ourutil.Reportf("Asking AWS for a certificate...")
	ccResp, err := iotSvc.CreateCertificateFromCsr(&iot.CreateCertificateFromCsrInput{
		CertificateSigningRequest: aws.String(pemBuf.String()),
		SetAsActive:               aws.Bool(true),
	})
	if err != nil {
		return nil, nil, errors.Annotatef(err, "failed to obtain certificate from AWS")
	}
	glog.Infof("AWS response:\n%s", cn)
	cpb, _ := pem.Decode([]byte(*ccResp.CertificatePem))
	if cpb == nil || cpb.Type != "CERTIFICATE" {
		return nil, nil, errors.Annotatef(err, "invalid cert data returned by AWS: %q", *ccResp.CertificatePem)
	}
	certDERBytes := cpb.Bytes
	x509utils.PrintCertInfo(certDERBytes)
	ourutil.Reportf("  ID      : %s", *ccResp.CertificateId)
	ourutil.Reportf("  ARN     : %s", *ccResp.CertificateArn)
	certPEMBytes := []byte(fmt.Sprintf("CN: %s\r\nID: %s\r\nARN: %s\r\n%s",
		cn, *ccResp.CertificateId, *ccResp.CertificateArn, *ccResp.CertificatePem))

	if policy != awsIoTPolicyNone {
		if policy == AWSIoTPolicyMOS {
			policies, err := GetAWSIoTPolicyNames(region, keyID, key)
			if err != nil {
				return nil, nil, errors.Trace(err)
			}
			found := false
			for _, p := range policies {
				if p == AWSIoTPolicyMOS {
					found = true
					break
				}
			}
			if !found {
				ourutil.Reportf("Creating policy %q (%s)...", AWSIoTPolicy, awsIoTPolicyMOSDocument)
				_, err := iotSvc.CreatePolicy(&iot.CreatePolicyInput{
					PolicyName:     aws.String(AWSIoTPolicy),
					PolicyDocument: aws.String(awsIoTPolicyMOSDocument),
				})
				if err != nil {
					return nil, nil, errors.Annotatef(err, "failed to create policy")
				}
			}
		}
		ourutil.Reportf("Attaching policy %q to the certificate...", policy)
		_, err := iotSvc.AttachPrincipalPolicy(&iot.AttachPrincipalPolicyInput{
			PolicyName: aws.String(AWSIoTPolicy),
			Principal:  ccResp.CertificateArn,
		})
		if err != nil {
			return nil, nil, errors.Annotatef(err, "failed to attach policy")
		}
	}

	if thingName != "-" {
		/* Try creating the thing, in case it doesn't exist. */
		_, err := iotSvc.CreateThing(&iot.CreateThingInput{
			ThingName: aws.String(thingName),
		})
		if err != nil && err.Error() != iot.ErrCodeResourceAlreadyExistsException {
			ourutil.Reportf("Error creating thing: %s", err)
			/*
			 * Don't fail right away, maybe we don't have sufficient permissions to
			 * create things but we can attach certs to existing things.
			 * If the thing does not exist, attaching will fail.
			 */
		}
		ourutil.Reportf("Attaching the certificate to %q...", thingName)
		_, err = iotSvc.AttachThingPrincipal(&iot.AttachThingPrincipalInput{
			ThingName: aws.String(thingName),
			Principal: ccResp.CertificateArn,
		})
		if err != nil {
			return nil, nil, errors.Annotatef(err, "failed to attach certificate to %q", thingName)
		}
	}

	return certPEMBytes, keyPEMBytes, nil
}

func StoreCreds(ak, sak string) (*credentials.Credentials, error) {
	sc := &credentials.SharedCredentialsProvider{}
	_, _ = sc.Retrieve() // This will fail, but we only need it to initialize Filename
	if sc.Filename == "" {
		return nil, errors.New("Could not determine file for cred storage")
	}
	cf, err := ini.Load(sc.Filename)
	if err != nil {
		cf = ini.Empty()
	}
	cf.Section("default").NewKey("aws_access_key_id", ak)
	cf.Section("default").NewKey("aws_secret_access_key", sak)

	os.MkdirAll(filepath.Dir(sc.Filename), 0700)
	if err = cf.SaveTo(sc.Filename); err != nil {
		return nil, errors.Annotatef(err, "failed to save %s", sc.Filename)
	}
	os.Chmod(sc.Filename, 0600)

	ourutil.Reportf("Wrote credentials to: %s", sc.Filename)

	// This should now succeed.
	creds := credentials.NewSharedCredentials("", "")
	_, err = creds.Get()
	if err != nil {
		os.Remove(sc.Filename)
		return nil, errors.Annotatef(err, "invalid new credentials")
	}
	return creds, nil
}

func askForCreds() (*credentials.Credentials, error) {
	ourutil.Reportf("\r\nAWS credentials are missing. If this is the first time you are running this tool,\r\n" +
		"you will need to obtain AWS credentials from the AWS console as explained here:\r\n" +
		"  http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-set-up.html\r\n")
	yn := ourutil.Prompt("Would you like to enter them now [y/N]?")
	if strings.ToUpper(yn) != "Y" {
		return nil, errors.New("user declined to enter creds")
	}
	ak := ourutil.Prompt("Access Key ID:")
	sak := ourutil.Prompt("Secret Access Key:")
	return StoreCreds(ak, sak)
}

func AWSIoTSetupFull(ctx context.Context, devConn dev.DevConn, region, policy, thing, keyID, key string) error {
	iotSvc, err := getSvc(region, keyID, key)
	if err != nil {
		return err
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
	mqttConf, err := devConf.Get("mqtt")
	if err != nil {
		return errors.Annotatef(err, "failed to get device MQTT config. Make sure firmware supports MQTT")
	}
	ourutil.Reportf("Current MQTT config: %+v", mqttConf)

	awsGGConf, err := devConf.Get("aws.greengrass")
	if err == nil {
		ourutil.Reportf("Current AWS Greengrass config: %+v", awsGGConf)
	}

	devID, err := devConf.Get("device.id")
	if err != nil {
		return errors.Annotatef(err, "failed to get device.id from config")
	}

	certCN := x509utils.CertCN
	if certCN == "" {
		certCN = devID
	}

	if thing == "" {
		awsIoTThing = certCN
	}

	certType, useATCA, err := x509utils.PickCertType(devInfo)
	if err != nil {
		return errors.Trace(err)
	}

	_, certPEMBytes, _, _, keyPEMBytes, err := x509utils.LoadCertAndKey(awsCertFile, awsKeyFile)

	if certPEMBytes == nil {
		certPEMBytes, keyPEMBytes, err = genCert(ctx, certType, useATCA, iotSvc, devConn, devConf, devInfo, certCN, awsIoTThing, region, policy, keyID, key)
		if err != nil {
			return errors.Annotatef(err, "failed to generate certificate")
		}
	}

	certDevFileName := ""
	if certPEMBytes != nil {
		certFileName := fmt.Sprintf("aws-%s.crt.pem", ourutil.FirstN(certCN, 16))
		certDevFileName, err = x509utils.WriteAndUploadFile(ctx, "certificate", certPEMBytes,
			awsCertFile, certFileName, devConn)
		if err != nil {
			return errors.Trace(err)
		}
	}
	keyDevFileName := ""
	if keyPEMBytes != nil {
		keyFileName := fmt.Sprintf("aws-%s.key.pem", ourutil.FirstN(certCN, 16))
		keyDevFileName, err = x509utils.WriteAndUploadFile(ctx, "key", keyPEMBytes,
			awsKeyFile, keyFileName, devConn)
		if err != nil {
			return errors.Trace(err)
		}
	} else if useATCA {
		keyDevFileName = fmt.Sprintf("%s%d", atca.KeyFilePrefix, x509utils.ATCASlot)
	} else {
		return errors.Errorf("BUG: no private key data!")
	}

	// ca.pem has both roots in it, so, for platforms other than CC3200, we can just use that.
	// CC3200 does not support cert bundles and will always require specific CA cert.
	caCertFile := "ca.pem"
	uploadCACert := false
	if strings.HasPrefix(strings.ToLower(*devInfo.Arch), "cc320") {
		caCertFile = rsaCACert
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

	settings := map[string]string{
		"mqtt.ssl_cert":    certDevFileName,
		"mqtt.ssl_key":     keyDevFileName,
		"mqtt.ssl_ca_cert": filepath.Base(caCertFile),
	}

	if awsMQTTServer == "" {
		// Get the value of mqtt.server from aws
		atsEPT := "iot:Data-ATS"
		de, err := iotSvc.DescribeEndpoint(&iot.DescribeEndpointInput{
			EndpointType: &atsEPT,
		})
		if err != nil {
			return errors.Annotatef(err, "aws iot describe-endpoint failed!")
		}
		settings["mqtt.server"] = fmt.Sprintf("%s:8883", *de.EndpointAddress)
	} else {
		settings["mqtt.server"] = awsMQTTServer
	}

	if useATCA {
		// ATECC508A makes ECDSA much faster than RSA, use it as first preference.
		settings["mqtt.ssl_cipher_suites"] =
			"TLS-ECDHE-ECDSA-WITH-AES-128-GCM-SHA256:TLS-RSA-WITH-AES-128-GCM-SHA256"
	}

	// MQTT requires device.id to be set.
	devId, err := devConf.Get("device.id")
	if devId == "" {
		settings["device.id"] = certCN
	}

	if thing != "-" {
		settings["aws.thing_name"] = thing
	}

	if awsGGEnable && awsGGConf != "" {
		settings["aws.greengrass.enable"] = "true"
		settings["mqtt.enable"] = "false"
	} else {
		settings["mqtt.enable"] = "true"
	}

	if err := config.ApplyDiff(devConf, settings); err != nil {
		return errors.Trace(err)
	}

	return config.SetAndSave(ctx, devConn, devConf)
}

func AWSIoTSetup(ctx context.Context, devConn dev.DevConn) error {
	return AWSIoTSetupFull(ctx, devConn, AWSRegion, AWSIoTPolicy, awsIoTThing, "", "")
}
