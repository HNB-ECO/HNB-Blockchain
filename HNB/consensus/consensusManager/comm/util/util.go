package util

import (
	"HNB/config"
	"HNB/consensus/algorand/types"
	"HNB/msp"
)

const (
	VP                 = "1"
	KEYTYPE_MEMBERSHIP = "membership"
)

func LoadTotalValidators() ([]*types.Validator, error) {
	totalVals := make([]*types.Validator, 0)
	for _, v := range config.Config.GeneValidators {
		address := msp.AccountPubkeyToAddress1(msp.StringToBccspKey(v.PubKeyStr))
		val := &types.Validator{
			PubKeyStr:   v.PubKeyStr,
			Address:     types.Address(address.GetBytes()),
			VotingPower: 1,
		}
		totalVals = append(totalVals, val)
	}

	return totalVals, nil
}
