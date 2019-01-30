package atca

type GenKeyArgs struct {
	Slot *int64 `json:"slot,omitempty"`
}

type GenKeyResult struct {
	Pubkey *string `json:"pubkey,omitempty"`
}

type GetConfigResult struct {
	Config *string `json:"config,omitempty"`
}

type GetPubKeyArgs struct {
	Slot *int64 `json:"slot,omitempty"`
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
	Ecc    *bool   `json:"ecc,omitempty"`
	Key    *string `json:"key,omitempty"`
	Slot   *int64  `json:"slot,omitempty"`
	Wkey   *string `json:"wkey,omitempty"`
	Wkslot *int64  `json:"wkslot,omitempty"`
}

type SignArgs struct {
	Digest *string `json:"digest,omitempty"`
	Slot   *int64  `json:"slot,omitempty"`
}

type SignResult struct {
	Signature *string `json:"signature,omitempty"`
}
