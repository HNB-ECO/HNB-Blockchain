package dbft

import (
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/contract/hnb"
)

const (
	TOKEN_THRESHOLD = 0.51
)

func (dbftMgr *DBFTManager) beginWitnessSelection(epoch *Epoch, validatorSet *types.ValidatorSet, epochCache *EpochListCache) ([]*types.Validator, uint64, error) {
	var witness []*types.Validator
	var err error
	var dependEpochNo uint64

	if dbftMgr.isVoteSatisfied(epoch.EpochNo) {
		witness, err = dbftMgr.getWitnessesByToken(epoch.EpochNo, validatorSet)
		if err != nil {
			return nil, 0, err
		}
		dependEpochNo = dbftMgr.candidateNo.GetCurrentNo()
	} else {
		witness, dependEpochNo, err = dbftMgr.getLastWitnesses(epoch.DependEpochNo, epochCache)
	}

	if err != nil {
		err = fmt.Errorf("witnessSelection err: %s\n", err)
		return nil, dependEpochNo, err
	}

	shuffledWitness := witnessShuffle(epoch.EpochNo, witness)
	ConsLog.Infof(LOGTABLE_DBFT, "(witness select) witness -> shuffled \n %v \n %v", witness, shuffledWitness)

	return shuffledWitness, dependEpochNo, err
}

func (dbftMgr *DBFTManager) isVoteSatisfied(epochNo uint64) bool {

	token, err := appMgr.GetVoteSum(epochNo)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "consensus vote sum err:%v", err.Error())
		return false
	}
	ConsLog.Debugf(LOGTABLE_DBFT, "consensus vote sum %v", token)
	// Judge whether token > 51% * total balance.
	if float32(token) > hnb.TOTAL_BALANCE*TOKEN_THRESHOLD {
		return true
	}

	return false
}

func (dbftMgr *DBFTManager) getWitnessesByToken(epochNo uint64, validatorSet *types.ValidatorSet) ([]*types.Validator, error) {
	var witnessList []*types.Validator

	peerIDs, err := appMgr.GetVotePeerIDOrderToken(epochNo)
	if err != nil {
		return nil, err
	}

	// Find top N nodes as witnesses.
	allValidators, err := dbftMgr.LoadTotalValidators(dbftMgr.candidateNo.GetCurrentNo())
	ConsLog.Debugf(LOGTABLE_DBFT, "allvalidators : %v", allValidators)
	ConsLog.Debugf(LOGTABLE_DBFT, "validatorSet.validators : %v", validatorSet.Validators)

	var validateNum uint8 = 0
	indexCache := make(map[int]struct{}, dbftMgr.BftNumber)
	for _, v := range peerIDs {
		if validateNum == dbftMgr.BftNumber {
			break
		}

		valIndex, validator := validatorSet.GetByPeerID(v)
		if valIndex == -1 {
			ConsLog.Warningf(LOGTABLE_DBFT, "(witness select) token peerID %v not found", v)
			continue
		}
		validateNum++
		witnessList = append(witnessList, validator)
		indexCache[valIndex] = struct{}{}
		ConsLog.Debugf(LOGTABLE_DBFT, "witness %d : %v", validateNum, validator)

	}

	// The number of witnesses should be N.
	for len(witnessList) != int(dbftMgr.BftNumber) {
		// 不够从当前白名单列表中补充，按照地址字节排序
		for valIndex, val := range validatorSet.Validators {
			_, ok := indexCache[valIndex]
			if !ok {
				witnessList = append(witnessList, val)
			}
		}
	}

	return witnessList, nil
}

func (dbftMgr *DBFTManager) getLastWitnesses(dependEpochNo uint64, epochCache *EpochListCache) ([]*types.Validator, uint64, error) {
	var newDependEpochNo uint64
	var index uint64 = 0

	// Zc add for debug
	fmt.Println("| Enter GetLastWitnesses Process...")

	if dependEpochNo > 0 {
		index = dependEpochNo - 1
	}
	for ; index > 0; index-- {
		if epochCache.isExist(index) {
			newDependEpochNo = index
			break
		}
	}
	return epochCache.GetEpoch(newDependEpochNo).WitnessList, newDependEpochNo, nil
}

func witnessShuffle(seed uint64, sList []*types.Validator) []*types.Validator {
	dList := make([]*types.Validator, len(sList))
	//for indexi, _ := range dList {
	//	dList[indexi] = &types.Validator{}
	//}
	for i, val := range sList {
		//dList[i] = types.NewValidator(val.PubKeyStr, val.Address, val.VotingPower, val.PeerID)
		//dList[i].Accum = val.Accum
		//dList[i] = &types.Validator{}
		dList[i] = val.Copy()
	}
	//seedHi := uint64(seed << 32)
	for i := 0; i < len(dList); i++ {
		// Use PRNG in http://xoshiro.di.unimi.it/
		k := uint64(seed) + uint64(i)*2685821657736338717
		k ^= k >> 12
		k ^= k << 25
		k ^= k >> 27
		k *= 2685821657736338717
		jMax := uint64(len(dList) - i)
		j := uint64(i) + k%jMax

		// Swap dList[i] and dList[j]
		//tmp := dList[i]
		//dList[i] = dList[j]
		//dList[j] = tmp
		dList[i], dList[j] = dList[j], dList[i]
	}
	return dList
}
