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
package atca

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"math/big"

	"github.com/juju/errors"

	"github.com/mongoose-os/mos/cli/dev"
	"github.com/mongoose-os/mos/cli/ourutil"
)

func GetPubKey(ctx context.Context, slot int, dc dev.DevConn) (*ecdsa.PublicKey, error) {
	req := &GetPubKeyArgs{Slot: int64(slot)}
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
	req := &GenKeyArgs{Slot: int64(slot)}

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
