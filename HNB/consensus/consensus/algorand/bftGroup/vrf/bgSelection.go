package vrf

import (
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/types"
	"crypto/sha512"
	"fmt"
)

func CalcBFTGroupMembersByVRF(VRFValue []byte, candidates []*cmn.BftGroupSwitchAdvice, groupSize int) ([]*cmn.BftGroupSwitchAdvice, error) {
	nodes := make([]*cmn.BftGroupSwitchAdvice, 0)
	nodeMap := make(map[uint32]bool)
	var ctr int

	// Candidate set should be larger than groupSize
	candidNum := len(candidates)
	if candidNum <= groupSize {
		err := fmt.Errorf("err: candidNum is equal or smaller than groupSize.\n")
		return candidates, err
	}

	// Calculate selection seed
	vrf := VRFValue

	// Loop end when adding enough validators
	for i := 0; ctr < groupSize; i++ {
		nodeID := CalcParticipant(vrf, candidNum, uint32(i))
		if _, present := nodeMap[nodeID]; !present {
			// Add new validator to list
			nodes = append(nodes, candidates[nodeID])
			nodeMap[nodeID] = true
			ctr++
		}
	}

	return nodes, nil
}

func CalcProposerByVRF(VRFValue []byte, valSet *types.ValidatorSet, height uint64, round int32) *types.Validator {
	index := (height + uint64(round)) % uint64(len(VRFValue))
	valIndex := CalcParticipant(VRFValue, valSet.Size(), uint32(index))
	_, val := valSet.GetByIndex(int(valIndex))

	return val
}

func CalcParticipant(vrf []byte, candidNum int, index uint32) uint32 {
	// Zc add in 18.08.14 and modified in 18.09.13 to achieve enhanced fairness
	// TODO: The fairness should be analysed.

	// Use SHA512(VRF) instead of using VRF itself.
	sha512VRF := sha512.Sum512(vrf)
	hash := sha512VRF[:]

	var v1, v2 uint32
	bIdx := index / 8
	bits1 := index % 8
	bits2 := 8 + bits1

	v1 = uint32(hash[bIdx]) >> bits1
	if bIdx+1 < uint32(len(hash)) {
		v2 = uint32(hash[bIdx+1])
	} else {
		v2 = uint32(hash[0])
	}

	v2 = v2 & ((1 << bits2) - 1)
	v := (v2 << (8 - bits1)) + v1
	v = v % uint32(candidNum)
	return v
}
