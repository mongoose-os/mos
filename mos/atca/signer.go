package atca

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/asn1"
	"encoding/base64"
	"io"
	"math/big"

	"cesanta.com/common/go/lptr"
	"cesanta.com/common/go/ourutil"
	"cesanta.com/mos/dev"
	"github.com/cesanta/errors"
)

// Implements crypto.Signer interface using ATCA.
type Signer struct {
	ctx  context.Context
	dc   dev.DevConn
	slot int
}

func NewSigner(ctx context.Context, dc dev.DevConn, slot int) crypto.Signer {
	return &Signer{ctx: ctx, dc: dc, slot: slot}
}

func (s *Signer) Public() crypto.PublicKey {
	pubk := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     big.NewInt(0),
		Y:     big.NewInt(0),
	}

	var r GetPubKeyResult
	if err := s.dc.Call(s.ctx, "ATCA.GetPubKey", &GetPubKeyArgs{
		Slot: lptr.Int64(int64(s.slot)),
	}, &r); err != nil {
		return nil
	}

	keyData, _ := base64.StdEncoding.DecodeString(*r.Pubkey)

	pubk.X.SetBytes(keyData[:PublicKeySize/2])
	pubk.Y.SetBytes(keyData[PublicKeySize/2 : PublicKeySize])

	return pubk
}

type ecdsaSignature struct {
	R, S *big.Int
}

func (s *Signer) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if len(digest) != 32 {
		return nil, errors.NotImplementedf("can only sign 32 byte digests, signing %d bytes", len(digest))
	}

	ourutil.Reportf("Signing with slot %d...\n", s.slot)

	b64d := base64.StdEncoding.EncodeToString(digest)
	var r SignResult
	if err := s.dc.Call(s.ctx, "ATCA.Sign", &SignArgs{
		Slot:   lptr.Int64(int64(s.slot)),
		Digest: &b64d,
	}, &r); err != nil {
		return nil, errors.Annotatef(err, "ATCA.Sign")
	}

	if r.Signature == nil {
		return nil, errors.New("no signature in response")
	}

	rawSig, _ := base64.StdEncoding.DecodeString(*r.Signature)
	if len(rawSig) != SignatureSize {
		return nil, errors.Errorf("invalid signature size: expected %d bytes, got %d",
			SignatureSize, len(rawSig))
	}

	sig := ecdsaSignature{
		R: big.NewInt(0),
		S: big.NewInt(0),
	}

	sig.R.SetBytes(rawSig[:SignatureSize/2])
	sig.S.SetBytes(rawSig[SignatureSize/2 : SignatureSize])

	return asn1.Marshal(sig)
}
