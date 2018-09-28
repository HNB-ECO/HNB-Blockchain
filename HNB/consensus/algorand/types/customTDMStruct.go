package types

import (
	"HNB/consensus/algorand/common"
)

type CustomTDMHeader struct {
	*Header       `json:"header"`
	Evidence      EvidenceData    `json:"evidence"`
	LastCommit    *Commit         `json:"last_commit"`
	DataHash      common.HexBytes `json:"dataHash"`
	ValidatorInfo *ValidatorInfo  `json:"validator_info"`
}

type CustomTDMExt struct {
	CurrentCommit *Commit `json:"current_commit"`
}

type BlkMaterial struct {
	//BatchID     []string //交易所在的batchID
	Height uint64
	NumTxs uint64 //交易个数，序列化把所有的交易序列化成了一个，所以要记录
	Txs    []Tx
	Commit *Commit //上一块的commit信息
	//BitMap      []*apiUtil.Bitmap
	Proposer    *Validator
	BlkVRFValue []byte
	BlkVRFProof []byte
}
