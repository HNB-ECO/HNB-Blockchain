package types

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
)

type CustomTDMHeader struct {
	*Header       `json:"header"`
	Evidence      EvidenceData   `json:"evidence"`
	LastCommit    *Commit        `json:"last_commit"`
	DataHash      util.HexBytes  `json:"dataHash"`
	ValidatorInfo *ValidatorInfo `json:"validator_info"`
}

type CustomTDMExt struct {
	CurrentCommit *Commit `json:"current_commit"`
}

type BlkMaterial struct {
	Height      uint64
	NumTxs      uint64
	Txs         []Tx
	Commit      *Commit
	Proposer    *Validator
	BlkVRFValue []byte
	BlkVRFProof []byte
}
