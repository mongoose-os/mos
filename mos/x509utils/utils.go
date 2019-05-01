package x509utils

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"

	"github.com/cesanta/errors"

	"cesanta.com/common/go/ourutil"
)

func WritePEM(derBytes []byte, blockType string, outputFileName string) error {
	var out io.Writer
	switch outputFileName {
	case "":
		out = os.Stdout
	case "-":
		out = os.Stdout
	case "--":
		out = os.Stderr
	default:
		f, err := os.OpenFile(outputFileName, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return errors.Annotatef(err, "failed to open %s for writing", outputFileName)
		}
		out = f
		defer func() {
			f.Close()
			ourutil.Reportf("Wrote %s", outputFileName)
		}()
	}
	pem.Encode(out, &pem.Block{Type: blockType, Bytes: derBytes})
	return nil
}

func WritePubKey(pubKey *ecdsa.PublicKey, outputFileName string) error {
	pubKeyDERBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return errors.Annotatef(err, "failed to marshal public key")
	}
	return WritePEM(pubKeyDERBytes, "PUBLIC KEY", outputFileName)
}
