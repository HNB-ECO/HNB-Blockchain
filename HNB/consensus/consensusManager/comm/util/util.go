package util

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
)

const (
	VP = "1"
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
