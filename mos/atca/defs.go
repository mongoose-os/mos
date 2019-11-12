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

type GenKeyArgs struct {
	Slot int64 `json:"slot"`
}

type GenKeyResult struct {
	Pubkey *string `json:"pubkey,omitempty"`
}

type GetConfigResult struct {
	Config *string `json:"config,omitempty"`
}

type GetPubKeyArgs struct {
	Slot int64 `json:"slot"`
}

type GetPubKeyResult struct {
	Pubkey *string `json:"pubkey,omitempty"`
}

type LockZoneArgs struct {
	Zone *int64 `json:"zone,omitempty"`
}

type SetConfigArgs struct {
	Config *string `json:"config,omitempty"`
}

type SetKeyArgs struct {
	Ecc    bool   `json:"ecc,omitempty"`
	Key    string `json:"key,omitempty"`
	Slot   int    `json:"slot"`
	Block  int    `json:"block,omitempty"`
	Wkey   string `json:"wkey,omitempty"`
	Wkslot int    `json:"wkslot,omitempty"`
}

type SignArgs struct {
	Digest *string `json:"digest,omitempty"`
	Slot   int64   `json:"slot"`
}

type SignResult struct {
	Signature *string `json:"signature,omitempty"`
}
