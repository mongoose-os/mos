package x509utils

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"cesanta.com/common/go/lptr"
	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/atca"
	"cesanta.com/mos/config"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/fs"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
)

const (
	defaultCertType  = "ECDSA"
	i2cEnableOption  = "i2c.enable"
	atcaEnableOption = "sys.atca.enable"
	rsaKeyBits       = 2048
)

type CertType string

const (
	CertTypeRSA         CertType      = "RSA"
	CertTypeECDSA                     = "ECDSA"
	defaultCertValidity time.Duration = (10*365*24 + 2*24) * time.Hour // ~10 years (+2 days for leap year adjustment)
)

var (
	certType     = ""
	CertCN       = ""
	useATCA      = false
	ATCASlot     = 0
	CertValidity time.Duration
)

func init() {
	flag.StringVar(&certType, "cert-type", "", "Type of the key for new cert, RSA or ECDSA. Default is "+defaultCertType+".")
	flag.StringVar(&CertCN, "cert-cn", "", "Common name for the certificate. By default uses device ID.")
	flag.DurationVar(&CertValidity, "cert-validity", defaultCertValidity, "Generated certificate validity")
	flag.BoolVar(&useATCA, "use-atca", false, "Use ATCA (ATECCx08A) to store private key.")
	flag.IntVar(&ATCASlot, "atca-slot", 0, "When using ATCA, use this slot for key storage.")
}

func PickCertType(devInfo *dev.GetInfoResult) (CertType, bool, error) {
	if useATCA {
		if certType != "" && strings.ToUpper(certType) != "ECDSA" {
			return "", true, errors.Errorf("ATCA only supports EC keys")
		}
		return CertTypeECDSA, true, nil
	}
	if certType != "" {
		switch strings.ToUpper(certType) {
		case "RSA":
			return CertTypeRSA, false, nil
		case "ECDSA":
			return CertTypeECDSA, false, nil
		default:
			return "", false, errors.Errorf("Invalid cert type %q", certType)
		}
	}
	// CC3200 seem to work better with RSA certs.
	// In all other cases (including CC3220), we prefer ECDSA.
	if strings.HasPrefix(strings.ToLower(*devInfo.Arch), "cc320") {
		return CertTypeRSA, false, nil
	}
	return CertTypeECDSA, false, nil
}

func checkATCAConfig(ctx context.Context, devConn dev.DevConn, devConf *dev.DevConf) error {
	i2cEnabled, err := devConf.Get(i2cEnableOption)
	if err != nil {
		return errors.Annotatef(err, "failed ot get I2C enabled status")
	}
	atcaEnabled, err := devConf.Get(atcaEnableOption)
	if err != nil {
		return errors.Annotatef(err, "failed to get ATCA enabled status")
	}
	if i2cEnabled != "true" || atcaEnabled != "true" {
		if i2cEnabled != "true" {
			ourutil.Reportf("Enabling I2C...")
			devConf.Set(i2cEnableOption, "true")
		}
		if atcaEnabled != "true" {
			ourutil.Reportf("Enabling ATCA...")
			devConf.Set(atcaEnableOption, "true")
		}
		err = config.SetAndSave(ctx, devConn, devConf)
		if err != nil {
			return errors.Annotatef(err, "failed to apply new configuration")
		}
		ourutil.Reportf("Reconnecting, please wait...")
		devConn.Disconnect(ctx)
		// Give the device time to reboot.
		time.Sleep(5 * time.Second)
		err = devConn.Connect(ctx, false)
		if err != nil {
			return errors.Annotatef(err, "failed to reconnect to the device")
		}
	}
	return nil
}

func GeneratePrivateKey(ctx context.Context, keyType CertType, useATCA bool, devConn dev.DevConn, devConf *dev.DevConf, devInfo *dev.GetInfoResult) (crypto.Signer, []byte, []byte, error) {
	var err error
	var keySigner crypto.Signer
	var keyPEMBlockType string
	var keyDERBytes, keyPEMBytes []byte

	ourutil.Reportf("Generating %s private key", keyType)

	if useATCA {
		if ATCASlot < 0 || ATCASlot > 7 {
			return nil, nil, nil, errors.Errorf("ATCA slot for private key must be between 0 and 7")
		}

		if err = checkATCAConfig(ctx, devConn, devConf); err != nil {
			// Don't fail outright, but warn the user
			ourutil.Reportf("invalid device configuration: %s", err)
		}

		_, atcaCfg, err := atca.Connect(ctx, devConn)
		if err != nil {
			return nil, nil, nil, errors.Annotatef(err, "failed to connect to the crypto device")
		}
		if atcaCfg.LockConfig != atca.LockModeLocked || atcaCfg.LockValue != atca.LockModeLocked {
			return nil, nil, nil, errors.Errorf(
				"chip is not fully configured; see step 1 here: " +
					"https://github.com/cesanta/mongoose-os-docs/blob/master/mos/userguide/security.md#setup-guide")
		}
		ourutil.Reportf("Generating new private key in slot %d", ATCASlot)
		if err = devConn.Call(ctx, "ATCA.GenKey", &atca.GenKeyArgs{
			Slot: lptr.Int64(int64(ATCASlot)),
		}, nil); err != nil {
			return nil, nil, nil, errors.Annotatef(err, "failed to generate private key in slot %d", ATCASlot)
		}
		keySigner = atca.NewSigner(ctx, devConn, ATCASlot)
		keyPEMBlockType = "EC PRIVATE KEY"
	} else {
		switch keyType {
		case CertTypeRSA:
			keySigner, err = rsa.GenerateKey(rand.Reader, rsaKeyBits)
			if err != nil {
				return nil, nil, nil, errors.Annotatef(err, "failed to generate EC private key")
			}
			keyPEMBlockType = "RSA PRIVATE KEY"
			keyDERBytes = x509.MarshalPKCS1PrivateKey(keySigner.(*rsa.PrivateKey))
		case CertTypeECDSA:
			keySigner, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return nil, nil, nil, errors.Annotatef(err, "failed to generate RSA private key")
			}
			keyPEMBlockType = "EC PRIVATE KEY"
			keyDERBytes, _ = x509.MarshalECPrivateKey(keySigner.(*ecdsa.PrivateKey))
		default:
			return nil, nil, nil, errors.Errorf("unknown key type %q", keyType)
		}
	}

	if keyDERBytes != nil {
		keyPEMBuf := bytes.NewBuffer(nil)
		pem.Encode(keyPEMBuf, &pem.Block{Type: keyPEMBlockType, Bytes: keyDERBytes})
		keyPEMBytes = keyPEMBuf.Bytes()
	}

	return keySigner, keyDERBytes, keyPEMBytes, nil
}

func PrintCertInfo(certDERBytes []byte) {
	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return
	}
	ourutil.Reportf("\nCertificate info:")
	ourutil.Reportf("  Subject : %s", cert.Subject)
	ourutil.Reportf("  Issuer  : %s", cert.Issuer)
	ourutil.Reportf("  Serial  : %s", cert.SerialNumber)
	ourutil.Reportf("  Validity: %s - %s",
		cert.NotBefore.Format("2006/01/02"), cert.NotAfter.Format("2006/01/02"))
	ourutil.Reportf("  Key algo: %s", cert.PublicKeyAlgorithm)
	ourutil.Reportf("  Sig algo: %s", cert.SignatureAlgorithm)
}

func LoadCertAndKey(certFile, keyFile string) ([]byte, []byte, crypto.Signer, []byte, []byte, error) {
	if certFile == "" || keyFile == "" {
		return nil, nil, nil, nil, nil, nil
	}
	var keySigner crypto.Signer
	var certDERBytes, certPEMBytes []byte
	var keyDERBytes, keyPEMBytes []byte
	certData, err1 := ioutil.ReadFile(certFile)
	keyData, err2 := ioutil.ReadFile(keyFile)
	if err1 == nil {
		cpb, _ := pem.Decode(certData)
		if cpb != nil {
			certDERBytes = cpb.Bytes
			certPEMBytes = certData
			ourutil.Reportf("Using existing cert: %s", certFile)
			kt := ""
			if !useATCA {
				var kpb *pem.Block
				if err2 == nil {
					kpb, _ = pem.Decode(keyData)
				}
				if kpb == nil {
					return nil, nil, nil, nil, nil, errors.Errorf("key file exists but not a key")
				}
				switch kpb.Type {
				case "RSA PRIVATE KEY":
					kt = "RSA"
					if keySigner, err1 = x509.ParsePKCS1PrivateKey(kpb.Bytes); err1 != nil {
						return nil, nil, nil, nil, nil, errors.Annotatef(err1, "invalid RSA private key %s", keyFile)
					}
				case "PRIVATE KEY":
					k, err1 := x509.ParsePKCS8PrivateKey(kpb.Bytes)
					if err1 != nil {
						return nil, nil, nil, nil, nil, errors.Annotatef(err1, "invalid private key %s", keyFile)
					}
					switch k.(type) {
					case *rsa.PrivateKey:
						kt = "RSA"
						keySigner = k.(*rsa.PrivateKey)
					case *ecdsa.PrivateKey:
						kt = "ECDSA"
						keySigner = k.(*ecdsa.PrivateKey)
					default:
						return nil, nil, nil, nil, nil, errors.Errorf("unknown key type %T in %s", k, keyFile)
					}
				case "EC PRIVATE KEY":
					kt = "ECDSA"
					if keySigner, err1 = x509.ParseECPrivateKey(kpb.Bytes); err1 != nil {
						return nil, nil, nil, nil, nil, errors.Annotatef(err1, "invalid ECDSA private key %s", keyFile)
					}
				default:
					return nil, nil, nil, nil, nil, errors.Errorf("unknown key format %s in %s", kpb.Type, keyFile)
				}
				keyDERBytes = kpb.Bytes
				keyPEMBytes = keyData
			} else {
				// XXX: this is not entirely correct, we should be returning a signer here...
				//keySigner = atca.NewSigner(ctx, cl, ATCASlot)
				kt = "ECDSA"
			}
			ourutil.Reportf("Using existing key : %s (%s)", keyFile, kt)
		}
	}
	return certDERBytes, certPEMBytes, keySigner, keyDERBytes, keyPEMBytes, nil
}

func LoadOrGenerateCertAndKey(ctx context.Context, certType CertType, certFile, keyFile string, certTmpl *x509.Certificate, useATCA bool,
	devConn dev.DevConn, devConf *dev.DevConf, devInfo *dev.GetInfoResult) (
	[]byte, []byte, crypto.Signer, []byte, []byte, error) {

	certDERBytes, certPEMBytes, keySigner, keyDERBytes, keyPEMBytes, err := LoadCertAndKey(certFile, keyFile)

	if certDERBytes == nil && certTmpl != nil {
		keySigner, keyDERBytes, keyPEMBytes, err = GeneratePrivateKey(ctx, certType, useATCA, devConn, devConf, devInfo)
		if err != nil {
			return nil, nil, nil, nil, nil, errors.Annotatef(err, "failed to generate private key")
		}
		if certTmpl.SerialNumber == nil {
			certTmpl.SerialNumber = big.NewInt(0)
		}
		if certTmpl.NotBefore.IsZero() && certTmpl.NotAfter.IsZero() {
			certTmpl.NotBefore = time.Now()
			certTmpl.NotAfter = certTmpl.NotBefore.Add(CertValidity)
		}
		certDERBytes, err = x509.CreateCertificate(rand.Reader, certTmpl, certTmpl, keySigner.Public(), keySigner)
		if err != nil {
			return nil, nil, nil, nil, nil, errors.Annotatef(err, "failed to generate certificate")
		}
	}
	if certPEMBytes == nil {
		pemBuf := bytes.NewBuffer(nil)
		pem.Encode(pemBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certDERBytes})
		certPEMBytes = pemBuf.Bytes()
	}
	PrintCertInfo(certDERBytes)
	return certDERBytes, certPEMBytes, keySigner, keyDERBytes, keyPEMBytes, nil
}

func WriteAndUploadFile(ctx context.Context,
	fileType string, data []byte, customName, defaultName string,
	devConn dev.DevConn) (string, error) {
	fileName := customName
	if fileName == "" {
		fileName = defaultName
	}
	ourutil.Reportf("Writing %s to %s...", fileType, fileName)
	if err := ioutil.WriteFile(fileName, data, 0600); err != nil {
		return "", errors.Annotatef(err, "failed to write %s to %s", fileType, fileName)
	}
	devFileName := ""
	if devConn != nil {
		devFileName = filepath.Base(fileName)
		ourutil.Reportf("Uploading %s (%d bytes)...", devFileName, len(data))
		if err := fs.PutData(ctx, devConn, bytes.NewBuffer(data), devFileName); err != nil {
			return "", errors.Annotatef(err, "failed to upload %s", devFileName)
		}
	}
	return devFileName, nil
}
