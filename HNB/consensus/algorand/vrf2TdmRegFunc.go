package algorand

import (
	appComm "HNB/appMgr/common"
	"HNB/config"
	"HNB/consensus/algorand/bftGroup/vrf"
	"HNB/consensus/algorand/msgHandler"
	"HNB/consensus/algorand/types"
	"HNB/msp"
	"HNB/txpool"
	"bytes"
)

func isProposerByVrf(h *msgHandler.TDMMsgHandler, height uint64, round int32) (bool, *types.Validator) {
	ConsLog.Debugf(LOGTABLE_CONS, "#(%v-%v) calc proposer preVrfValue %x, vals %s", height, round, h.LastCommitState.PrevVRFValue, h.Validators)

	proposer := vrf.CalcProposerByVRF(h.LastCommitState.PrevVRFValue, h.Validators, height, round)
	return bytes.Equal(h.GetDigestAddr(), proposer.Address), proposer
}

func geneProposalBlkByVrf(h *msgHandler.TDMMsgHandler) (block *types.Block, blockParts *types.PartSet) {
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
		ConsLog.Errorf(LOGTABLE_CONS, "#(%v-%v) enterPropose: Cannot propose anything: No commit for the previous block.", h.Height, h.Round)
		return
	}

	var txs []types.Tx
	txPool, err := txpool.GetTxsFromTXPool(appComm.HNB, 1000)
	if err != nil || len(txPool) == 0 {
		ConsLog.Warningf(LOGTABLE_CONS, "enterPropose: empty txs")
		if !config.Config.CreateEmptyBlocks {
			ConsLog.Infof(LOGTABLE_CONS, "config not create empty")
			return
		}
		txs = make([]types.Tx, 0)
	} else {
		txs, err = types.Tx2TDMTx(txPool)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "enterPropose: get original tx err %v", err)
			return
		}
		ConsLog.Infof(LOGTABLE_CONS, "propose tx len %v", len(txs))
	}

	_, val := h.Validators.GetByAddress(h.GetDigestAddr())

	VRFValue, VRFProof, err := h.ComputeNewVRF()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "%s", err)
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

	block, parts := h.LastCommitState.MakeBlockVRF(blkMaterial)
	return block, parts
}

func validateProposalBlkByVrf(h *msgHandler.TDMMsgHandler, proposalBlk *types.Block) bool {
	proposer := proposalBlk.Validators.Proposer
	VRFValue := proposalBlk.BlkVRFValue
	VRFProof := proposalBlk.BlkVRFProof

	VRFBlkData := &vrf.VRFBlkData{
		PrevVrf:  h.LastCommitState.PrevVRFValue,
		BlockNum: proposalBlk.BlockNum,
	}

	_, val := h.Validators.GetByAddress(proposer.Address)
	pk := msp.StringToBccspKey(val.PubKeyStr)

	VRFVerifySuccess, err := vrf.VerifyVRF4Blk(pk, VRFBlkData, VRFValue, VRFProof, msp.GetAlgType())
	if err != nil {
		return false
	}

	if !VRFVerifySuccess {
		return false
	}
	return true
}
