package msgHandler

import (
	"bytes"
	"HNB/consensus/algorand/common"
	"HNB/consensus/algorand/types"
)

type BftGroup struct {
	BgID uint64
	Validators []*types.Validator
}

func (bg BftGroup) Exist(digestAddr common.HexBytes) bool {
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
