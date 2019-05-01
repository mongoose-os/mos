package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"cesanta.com/mos/atca"
	"cesanta.com/mos/dev"
	"cesanta.com/mos/flags"
	"cesanta.com/mos/x509utils"
	"github.com/cesanta/errors"
	flag "github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"
)

var (
	csrTemplate string
)

func getFormat(f, fn string) string {
	f = strings.ToLower(f)
	if f == "" {
		fn := strings.ToLower(fn)
		if strings.HasSuffix(fn, ".yaml") || strings.HasSuffix(fn, ".yml") {
			f = "yaml"
		} else if strings.HasSuffix(strings.ToLower(fn), ".json") {
			f = "json"
		} else {
			f = "hex"
		}
	}
	return f
}

func atcaGetConfig(ctx context.Context, dc dev.DevConn) error {
	fn := ""
	args := flag.Args()
	if len(args) == 2 {
		fn = args[1]
	}

	confData, cfg, err := atca.Connect(ctx, dc)
	if err != nil {
		return errors.Annotatef(err, "Connect")
	}

	f := getFormat(*flags.Format, fn)

	var s []byte
	if f == "json" || f == "yaml" {
		if f == "json" {
			s, _ = json.MarshalIndent(cfg, "", "  ")
		} else {
			s, _ = yaml.Marshal(cfg)
		}
	} else if f == "hex" {
		s = atca.WriteHex(confData, 4)
	} else {
		return errors.Errorf("%s: format not specified and could not be guessed", fn)
	}

	if fn != "" {
		err = ioutil.WriteFile(fn, s, 0644)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		os.Stdout.Write(s)
	}

	return nil
}

func atcaSetConfig(ctx context.Context, dc dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("config filename is required")
	}
	fn := args[1]

	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return errors.Trace(err)
	}

	f := getFormat(*flags.Format, fn)

	var confData []byte
	if f == "yaml" || f == "json" {
		var c atca.Config
		if f == "yaml" {
			err = yaml.Unmarshal(data, &c)
		} else {
			err = json.Unmarshal(data, &c)
		}
		if err != nil {
			return errors.Annotatef(err, "failed to decode %s as %s", fn, f)
		}

		confData, err = atca.WriteBinaryConfig(&c)
		if err != nil {
			return errors.Annotatef(err, "encode %s", fn)
		}
	} else if f == "hex" {
		confData = atca.ReadHex(data)
	} else {
		return errors.Errorf("%s: format not specified and could not be guessed", fn)
	}

	if len(confData) != atca.ConfigSize {
		return errors.Errorf("%s: expected %d bytes, got %d", fn, atca.ConfigSize, len(confData))
	}

	_, currentCfg, err := atca.Connect(ctx, dc)
	if err != nil {
		return errors.Annotatef(err, "Connect")
	}

	if currentCfg.LockConfig == atca.LockModeLocked {
		return errors.Errorf("config zone is already locked")
	}

	b64c := base64.StdEncoding.EncodeToString(confData)
	req := &atca.SetConfigArgs{
		Config: &b64c,
	}

	if *dryRun {
		reportf("This is a dry run, would have set the following config:\n\n"+
			"%s\n"+
			"SetConfig %s\n\n"+
			"Set --dry-run=false to confirm.",
			atca.WriteHex(confData, 4), atca.JSONStr(*req))
		return nil
	}

	if err = dc.Call(ctx, "ATCA.SetConfig", req, nil); err != nil {
		return errors.Annotatef(err, "SetConfig")
	}

	reportf("\nSetConfig successful.")

	return nil
}

func atcaLockZone(ctx context.Context, dc dev.DevConn) error {
	args := flag.Args()
	if len(args) != 2 {
		return errors.Errorf("lock zone name is required (config or data)")
	}

	var zone atca.LockZone
	switch strings.ToLower(args[1]) {
	case "config":
		zone = atca.LockZoneConfig
	case "data":
		zone = atca.LockZoneData
	default:
		return errors.Errorf("invalid zone '%s'", args[1])
	}

	_, _, err := atca.Connect(ctx, dc)
	if err != nil {
		return errors.Annotatef(err, "Connect")
	}

	zoneInt := int64(zone)
	req := &atca.LockZoneArgs{Zone: &zoneInt}

	if *dryRun {
		reportf("This is a dry run, would have sent the following request:\n\n"+
			"LockZone %s\n\n"+
			"Set --dry-run=false to confirm.", atca.JSONStr(req))
		return nil
	}

	if err = dc.Call(ctx, "ATCA.LockZone", req, nil); err != nil {
		return errors.Annotatef(err, "LockZone")
	}

	reportf("LockZone successful.")

	return nil
}

func atcaSetECCPrivateKey(slot int64, cfg *atca.Config, data []byte) (*atca.SetKeyArgs, error) {
	var keyData []byte

	rest := data
	for {
		var pb *pem.Block
		pb, rest = pem.Decode(rest)
		if pb != nil {
			if pb.Type != "EC PRIVATE KEY" {
				continue
			}
			eck, err := x509.ParseECPrivateKey(pb.Bytes)
			if err != nil {
				return nil, errors.Annotatef(err, "ParseECPrivateKey")
			}
			reportf("Parsed %s", pb.Type)
			keyData = eck.D.Bytes()
			break
		} else {
			keyData = atca.ReadHex(data)
			break
		}
	}

	if len(keyData) == atca.PrivateKeySize+1 && keyData[0] == 0 {
		// Copy-pasted from X509, chop off leading 0.
		keyData = keyData[1:]
	}

	if len(keyData) > atca.PrivateKeySize {
		return nil, errors.Errorf("expected %d bytes, got %d", atca.PrivateKeySize, len(keyData))
	}

	b64k := base64.StdEncoding.EncodeToString(keyData)
	isECC := true
	req := &atca.SetKeyArgs{Key: &b64k, Ecc: &isECC}

	if cfg.LockValue == atca.LockModeLocked {
		if cfg.SlotInfo[slot].SlotConfig.WriteConfig&0x4 == 0 {
			return nil, errors.Errorf(
				"data zone is locked and encrypted writes on slot %d "+
					"are not enabled, key cannot be set", slot)
		}
		wks := int64(cfg.SlotInfo[slot].SlotConfig.WriteKey)
		if *flags.WriteKey == "" {
			return nil, errors.Errorf(
				"data zone is locked, --write-key for slot %d "+
					"is required to modify slot %d", wks, slot)
		}
		reportf("Data zone is locked, "+
			"will perform encrypted write using slot %d using %s", wks, *flags.WriteKey)
		wKeyData, err := ioutil.ReadFile(*flags.WriteKey)
		if err != nil {
			return nil, errors.Trace(err)
		}
		wKey := atca.ReadHex(wKeyData)
		if len(wKey) != atca.KeySize {
			return nil, errors.Errorf("%s: expected %d bytes, got %d", *flags.WriteKey, atca.KeySize, len(wKey))
		}
		b64wk := base64.StdEncoding.EncodeToString(wKey)
		req.Wkslot = &wks
		req.Wkey = &b64wk
	}

	return req, nil
}

func atcaSetKey(ctx context.Context, dc dev.DevConn) error {
	args := flag.Args()
	if len(args) != 3 {
		return errors.Errorf("slot number and key filename are required")
	}
	slot, err := strconv.ParseInt(args[1], 0, 64)
	if err != nil || slot < 0 || slot > 15 {
		return errors.Errorf("invalid slot number %q", args[1])
	}

	fn := args[2]

	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return errors.Trace(err)
	}

	_, cfg, err := atca.Connect(ctx, dc)
	if err != nil {
		return errors.Annotatef(err, "Connect")
	}

	if cfg.LockConfig != atca.LockModeLocked {
		return errors.Errorf("config zone must be locked got SetKey to work")
	}

	var req *atca.SetKeyArgs

	si := cfg.SlotInfo[slot]
	if slot < 8 && si.KeyConfig.Private && si.KeyConfig.KeyType == atca.KeyTypeECC {
		reportf("Slot %d is a ECC private key slot", slot)
		req, err = atcaSetECCPrivateKey(slot, cfg, data)
	} else {
		reportf("Slot %d is a non-ECC private key slot", slot)
		keyData := atca.ReadHex(data)
		if len(keyData) != atca.KeySize {
			return errors.Errorf("%s: expected %d bytes, got %d", fn, atca.KeySize, len(keyData))
		}
		b64k := base64.StdEncoding.EncodeToString(keyData)
		isECC := false
		req = &atca.SetKeyArgs{Key: &b64k, Ecc: &isECC}
	}

	if err != nil {
		return errors.Annotatef(err, fn)
	}

	keyData, _ := base64.StdEncoding.DecodeString(*req.Key)
	req.Slot = &slot

	if *dryRun {
		reportf("This is a dry run, would have set the following key on slot %d:\n\n%s\n"+
			"SetKey %s\n\n"+
			"Set --dry-run=false to confirm.",
			slot, atca.WriteHex(keyData, 16), atca.JSONStr(*req))
		return nil
	}

	if err = dc.Call(ctx, "ATCA.SetKey", req, nil); err != nil {
		return errors.Annotatef(err, "SetKey")
	}

	reportf("SetKey successful.")

	return nil
}

func atcaGenKey(ctx context.Context, dc dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("slot number is required")
	}
	slot, err := strconv.ParseInt(args[1], 0, 64)
	if err != nil || slot < 0 || slot > 15 {
		return errors.Errorf("invalid slot number %q", args[1])
	}

	outputFileName := ""
	if len(args) == 3 {
		outputFileName = args[2]
	}

	if _, _, err := atca.Connect(ctx, dc); err != nil {
		return errors.Annotatef(err, "Connect")
	}
	pubKeyData, err := atca.GenKey(ctx, int(slot), *dryRun, dc)
	if err != nil {
		return errors.Trace(err)
	}
	if pubKeyData == nil { // dry run
		return nil
	}

	return x509utils.WritePubKey(pubKeyData, outputFileName)
}

func genCSR(ctx context.Context, csrTemplateFile string, subject string, slot int, dc dev.DevConn, outputFileName string) ([]byte, error) {
	if csrTemplateFile == "" && subject == "" {
		return nil, errors.Errorf("CSR template file or subject is required")
	}
	var csrTemplate *x509.CertificateRequest
	if csrTemplateFile != "" {
		reportf("Generating CSR using template from %s", csrTemplateFile)
		data, err := ioutil.ReadFile(csrTemplateFile)
		if err != nil {
			return nil, errors.Trace(err)
		}

		var pb *pem.Block
		pb, _ = pem.Decode(data)
		if pb == nil {
			return nil, errors.Errorf("%s: not a PEM file", csrTemplateFile)
		}
		if pb.Type != "CERTIFICATE REQUEST" {
			return nil, errors.Errorf("%s: expected to find certificate request, found %s", csrTemplateFile, pb.Type)
		}
		csrTemplate, err = x509.ParseCertificateRequest(pb.Bytes)
		if err != nil {
			return nil, errors.Annotatef(err, "%s: failed to parse certificate request template", csrTemplateFile)
		}
	} else {
		// Create a simple CSR.
		csrTemplate = &x509.CertificateRequest{
			PublicKeyAlgorithm: x509.ECDSA,
			SignatureAlgorithm: x509.ECDSAWithSHA256,
		}
	}
	if subject != "" {
		dn, err := x509utils.ParseDN(subject)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid subject %q", subject)
		}
		subj, err := dn.ToPKIXName()
		if err != nil {
			return nil, errors.Annotatef(err, "invalid subject %q", subject)
		}
		csrTemplate.Subject = *subj
	}
	reportf("Subject: %s", csrTemplate.Subject.ToRDNSequence())
	if csrTemplate.PublicKeyAlgorithm != x509.ECDSA ||
		csrTemplate.SignatureAlgorithm != x509.ECDSAWithSHA256 {
		return nil, errors.Errorf("%s: wrong public key and/or signature type; "+
			"expected ECDSA(%d) and SHA256(%d), got %d %d",
			csrTemplateFile,
			x509.ECDSA, x509.ECDSAWithSHA256, csrTemplate.PublicKeyAlgorithm,
			csrTemplate.SignatureAlgorithm)
	}
	signer := atca.NewSigner(ctx, dc, slot)
	csrData, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, signer)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to create new CSR")
	}

	return csrData, nil
}

func atcaGenCSR(ctx context.Context, dc dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("slot number is required")
	}
	slot, err := strconv.ParseInt(args[1], 0, 64)
	if err != nil || slot < 0 || slot > 15 {
		return errors.Errorf("invalid slot number %q", args[1])
	}
	outputFileName := ""
	if len(args) == 3 {
		outputFileName = args[2]
	}
	if *flags.CSRTemplate == "" && *flags.Subject == "" {
		return errors.Errorf("--csr-template or --subject is required")
	}

	if _, _, err := atca.Connect(ctx, dc); err != nil {
		return errors.Annotatef(err, "Connect")
	}
	pubKeyData, err := atca.GenKey(ctx, int(slot), *dryRun, dc)
	if err != nil {
		return errors.Trace(err)
	}
	if pubKeyData == nil { // dry run
		return nil
	}

	csrData, err := genCSR(ctx, *flags.CSRTemplate, *flags.Subject, int(slot), dc, outputFileName)
	if err != nil {
		return errors.Annotatef(err, "genCSR")
	}

	return x509utils.WritePEM(csrData, "CERTIFICATE REQUEST", outputFileName)
}

func genCert(ctx context.Context, certTemplateFile string, subject string, validityDays int, slot int, caCert *x509.Certificate, caSigner crypto.Signer, dc dev.DevConn, outputFileName string) ([]byte, error) {
	if certTemplateFile == "" && subject == "" {
		return nil, errors.Errorf("cert template file or subject is required")
	}
	if !caCert.IsCA {
		return nil, errors.Errorf("signing cert is not a CA cert")
	}
	var certTemplate *x509.Certificate
	if certTemplateFile != "" {
		reportf("Generating cert using template from %s", certTemplateFile)
		data, err := ioutil.ReadFile(certTemplateFile)
		if err != nil {
			return nil, errors.Trace(err)
		}
		var pb *pem.Block
		pb, _ = pem.Decode(data)
		if pb == nil {
			return nil, errors.Errorf("%s: not a PEM file", certTemplateFile)
		}
		if pb.Type != "CERTIFICATE" {
			return nil, errors.Errorf("%s: expected to find certificate, found %s", certTemplateFile, pb.Type)
		}
		certTemplate, err = x509.ParseCertificate(pb.Bytes)
		if err != nil {
			return nil, errors.Annotatef(err, "%s: failed to parse certificate template", certTemplateFile)
		}
	} else {
		// Create a simple cert.
		sn, _ := rand.Int(rand.Reader, big.NewInt(1<<63-1))
		certTemplate = &x509.Certificate{
			SerialNumber:       sn,
			PublicKeyAlgorithm: x509.ECDSA,
			SignatureAlgorithm: x509.ECDSAWithSHA256,
			KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage: []x509.ExtKeyUsage{
				x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth,
			},
			IsCA: false,
			BasicConstraintsValid: true,
		}
	}
	if subject != "" {
		dn, err := x509utils.ParseDN(subject)
		if err != nil {
			return nil, errors.Annotatef(err, "invalid subject %q", subject)
		}
		subj, err := dn.ToPKIXName()
		if err != nil {
			return nil, errors.Annotatef(err, "invalid subject %q", subject)
		}
		certTemplate.Subject = *subj
	}
	if validityDays > 0 {
		certTemplate.NotBefore = time.Now()
		certTemplate.NotAfter = time.Now().Add(time.Duration(validityDays*24) * time.Hour)
	}
	if time.Now().After(certTemplate.NotAfter) {
		return nil, errors.Errorf("invalid certificate validity, must be provided by template or --cert-days")
	}
	reportf("Subject: %s", certTemplate.Subject.ToRDNSequence())
	if certTemplate.PublicKeyAlgorithm != x509.ECDSA ||
		certTemplate.SignatureAlgorithm != x509.ECDSAWithSHA256 {
		return nil, errors.Errorf("%s: wrong public key and/or signature type; "+
			"expected ECDSA(%d) and SHA256(%d), got %d %d",
			certTemplateFile,
			x509.ECDSA, x509.ECDSAWithSHA256, certTemplate.PublicKeyAlgorithm,
			certTemplate.SignatureAlgorithm)
	}
	pubKey, err := atca.GetPubKey(ctx, slot, dc)
	if err != nil {
		return nil, errors.Annotatef(err, "GetPubKey")
	}
	certData, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, pubKey, caSigner)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to create new certificate")
	}
	return certData, nil
}

func atcaGenCert(ctx context.Context, dc dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("slot number is required")
	}
	slot, err := strconv.ParseInt(args[1], 0, 64)
	if err != nil || slot < 0 || slot > 15 {
		return errors.Errorf("invalid slot number %q", args[1])
	}
	if *flags.CAFile == "" && *flags.CAKeyFile == "" {
		return errors.Errorf("--ca-cert-file and --ca-key-file are required")
	}
	outputFileName := ""
	if len(args) == 3 {
		outputFileName = args[2]
	}

	if *flags.CertTemplate == "" && *flags.Subject == "" {
		return errors.Errorf("--cert-template or --subject is required")
	}
	reportf("Signing certificate:")
	caCertDERBytes, _, caSigner, _, _, err := x509utils.LoadCertAndKey(*flags.CAFile, *flags.CAKeyFile)
	if err != nil {
		return errors.Annotatef(err, "failed to load signing cert")
	}
	caCert, err := x509.ParseCertificate(caCertDERBytes)
	if err != nil {
		return errors.Annotatef(err, "invalid signing cert")
	}

	if _, _, err := atca.Connect(ctx, dc); err != nil {
		return errors.Annotatef(err, "Connect")
	}
	pubKey, err := atca.GenKey(ctx, int(slot), *dryRun, dc)
	if err != nil {
		return errors.Trace(err)
	}
	if pubKey == nil { // dry run
		return nil
	}

	certData, err := genCert(ctx, *flags.CertTemplate, *flags.Subject, *flags.CertDays, int(slot), caCert, caSigner, dc, outputFileName)
	if err != nil {
		return errors.Annotatef(err, "genCert")
	}

	_, err = x509utils.WriteAndUploadFile(ctx, "certificate", certData, outputFileName, fmt.Sprintf("atca-%d.crt.pem", slot), dc)
	return err
}

func atcaGetPubKey(ctx context.Context, dc dev.DevConn) error {
	args := flag.Args()
	if len(args) < 2 {
		return errors.Errorf("slot number is required")
	}
	slot, err := strconv.ParseInt(args[1], 0, 64)
	if err != nil || slot < 0 || slot > 15 {
		return errors.Errorf("invalid slot number %q", args[1])
	}
	outputFileName := ""
	if len(args) == 3 {
		outputFileName = args[2]
	}
	if _, _, err := atca.Connect(ctx, dc); err != nil {
		return errors.Annotatef(err, "Connect")
	}
	pubKey, err := atca.GetPubKey(ctx, int(slot), dc)
	if err != nil {
		return errors.Annotatef(err, "getPubKey")
	}
	return x509utils.WritePubKey(pubKey, outputFileName)
}
