package msgHandler

import (
	"HNB/consensus/algorand/types"
	"HNB/util"
	"bytes"
)

type BftGroup struct {
	BgID uint64
	// VRF
	VRFValue []byte
	VRFProof []byte

	Validators []*types.Validator
}

func (bg BftGroup) Exist(digestAddr util.HexBytes) bool {
	if len(digestAddr) == 0 {
		return false
	}

	for _, validator := range bg.Validators {
		if bytes.Equal(validator.Address, digestAddr) {
			return true
		}
	}

	return false
}
