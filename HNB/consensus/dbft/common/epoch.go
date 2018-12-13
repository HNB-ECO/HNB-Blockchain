package common

import "HNB/consensus/algorand/types"

//  todo
func GetEpochWitnessesLen(epochNo uint64) int {
	return 0
}

// todo  获取见证人列表
func GetEpochWitnesses(epochNo uint64) []*types.Validator {
	return nil
}

// todo
func GetEpochInfo(epochNo uint64) ([]*types.Validator, uint64) {
	return nil, 0
}

//todo
func GetNewEpochWitnesses(epochNo uint64, count int) ([]*types.Validator, error) {
	return nil, nil
}

//todo
func GetAllToken() uint64 {
	return 0
}

func GetEpocFrozenToken(epochNo uint64) uint64 {
	return 0
}
