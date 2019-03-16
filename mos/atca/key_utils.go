package atca

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"math/big"

	"github.com/cesanta/errors"

	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/dev"
)

func GetPubKey(ctx context.Context, slot int, dc dev.DevConn) (*ecdsa.PublicKey, error) {
	slot64 := int64(slot)
	req := &GetPubKeyArgs{Slot: &slot64}
	var r GetPubKeyResult
	if err := dc.Call(ctx, "ATCA.GetPubKey", req, &r); err != nil {
		return nil, errors.Annotatef(err, "GetPubKey")
	}
	if r.Pubkey == nil {
		return nil, errors.New("no public key in response")
	}
	pubKeyData, err := base64.StdEncoding.DecodeString(*r.Pubkey)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to decode pub key data")
	}
	if len(pubKeyData) != PublicKeySize {
		return nil, errors.Errorf("expected %d bytes, got %d", PublicKeySize, len(pubKeyData))
	}
	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     big.NewInt(0).SetBytes(pubKeyData[0:32]),
		Y:     big.NewInt(0).SetBytes(pubKeyData[32:64]),
	}
	return pubKey, nil
}

func GenKey(ctx context.Context, slot int, dryRun bool, dc dev.DevConn) (*ecdsa.PublicKey, error) {
	slot64 := int64(slot)
	req := &GenKeyArgs{Slot: &slot64}

	if dryRun {
		ourutil.Reportf("This is a dry run, would have sent the following request:\n\n"+
			"GenKey %s\n\n"+
			"Set --dry-run=false to confirm.",
			JSONStr(*req))
		return nil, nil
	}

	var r GenKeyResult
	if err := dc.Call(ctx, "ATCA.GenKey", req, &r); err != nil {
		return nil, errors.Annotatef(err, "GenKey")
	}

	if r.Pubkey == nil {
		return nil, errors.New("no public key in response")
	}

	pubKeyData, err := base64.StdEncoding.DecodeString(*r.Pubkey)
	if err != nil {
		return nil, errors.Annotatef(err, "failed to decode pub key data")
	}
	if len(pubKeyData) != PublicKeySize {
		return nil, errors.Errorf("expected %d bytes, got %d", PublicKeySize, len(pubKeyData))
	}
	ourutil.Reportf("Generated new ECC key on slot %d", slot)
	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     big.NewInt(0).SetBytes(pubKeyData[0:32]),
		Y:     big.NewInt(0).SetBytes(pubKeyData[32:64]),
	}
	return pubKey, nil
}
