package dbft

import (
	"bytes"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr"
	appComm "github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/msgHandler"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/txpool"
)

func (dbftMgr *DBFTManager) isProposerByDPoS(h *msgHandler.TDMMsgHandler, height uint64, round int32) (bool, *types.Validator) {
	// Get validators in this round.
	currentEpochNo := dbftMgr.epoch.EpochNo

	currentEpoch := dbftMgr.epochList.GetEpoch(currentEpochNo)
	if currentEpoch == nil {
		return false, nil
	}
	ConsLog.Infof(LOGTABLE_DBFT, "isProposerByDPoS currentEpoch %v", currentEpoch)
	validators := currentEpoch.WitnessList
	if len(validators) == 0 {
		return false, nil
	}

	var newProposer *types.Validator
	var valIndex int
	if height == dbftMgr.epoch.BeginNum {
		newProposer = validators[0]
		valIndex = 0
	} else {
		// Get newProposer in last block.
		lastProposer := h.LastValidators.Proposer
		ConsLog.Debugf(LOGTABLE_DBFT, "#(%d-%d-%v) round last: %v \n state last:%v",
			h.Height, h.Round, h.Step, h.LastValidators, h.LastCommitState.LastValidators)
		lastIndex, _ := h.LastValidators.GetByAddress(lastProposer.Address)
		ConsLog.Infof(LOGTABLE_DBFT, "#(%d-%d-%v) last proposer: %v, index %d", h.Height, h.Round, h.Step, lastProposer.PubKeyStr, lastIndex)

		valIndex = lastIndex + 1
		if valIndex == len(validators) {
			valIndex = 0
		}
	}

	// Find newProposer i this round by index.
	newProposer = validators[(valIndex+int(round))%len(validators)]

	ConsLog.Infof(LOGTABLE_DBFT, "#(%d-%d) blkProposer: %s",
		height, round, newProposer.PubKeyStr)

	return bytes.Equal(h.GetDigestAddr(), newProposer.Address), newProposer
}

func geneProposalBlkByDPoS(h *msgHandler.TDMMsgHandler) (block *types.Block, blockParts *types.PartSet) {
	var commit *types.Commit
	if h.Height == 1 {
		// We're creating a proposal for the first block.
		// The commit is empty, but not nil.
		commit = &types.Commit{}
	} else if h.LastCommit.HasTwoThirdsMajority() {
		// Make the commit from LastCommit
		commit = h.LastCommit.MakeCommit()
	} else {
		// This shouldn't happen.
		ConsLog.Errorf(LOGTABLE_DBFT, "#(%v-%v) enterPropose: Cannot propose anything: No commit for the previous block.", h.Height, h.Round)
		return
	}

	var txs []types.Tx
	txPool, err := txpool.GetTxsFromTXPool(appComm.HNB, 1000)
	if err != nil || len(txPool) == 0 {
		ConsLog.Warningf(LOGTABLE_DBFT, "enterPropose: empty txs")
		if !config.Config.CreateEmptyBlocks {
			ConsLog.Infof(LOGTABLE_DBFT, "config not create empty")
			return
		}
		txs = make([]types.Tx, 0)
	} else {
		txs, err = types.Tx2TDMTx(txPool)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "enterPropose: get original tx err %v", err)
			return
		}
		ConsLog.Infof(LOGTABLE_DBFT, "propose tx len %v", len(txs))
	}

	_, val := h.Validators.GetByAddress(h.GetDigestAddr())

	VRFValue, VRFProof, err := h.ComputeNewVRF()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "%s", err)
	}

	blkMaterial := &types.BlkMaterial{
		BlkVRFProof: VRFProof,
		BlkVRFValue: VRFValue,
		Height:      h.Height,
		Proposer:    val,
		Commit:      commit,
		NumTxs:      uint64(len(txs)),
		Txs:         txs,
	}

	ConsLog.Infof(LOGTABLE_DBFT, "geneProposalBlkByDPoS LastCommitState extend %v", h.LastCommitState.Validators.Extend)
	block, parts := h.LastCommitState.MakeBlockDPoS(blkMaterial)
	return block, parts
}

func validateProposalBlkByDPoS(h *msgHandler.TDMMsgHandler, proposalBlk *types.Block) bool {
	return true
}

func (dbftMgr *DBFTManager) unfreezeToken(epochNo uint64) error {
	return appMgr.UnfreezeToken(epochNo)
}
