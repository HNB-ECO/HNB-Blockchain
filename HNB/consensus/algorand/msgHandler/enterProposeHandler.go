package msgHandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	appComm "github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/bftGroup/vrf"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/txpool"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
)

func (h *TDMMsgHandler) enterPropose(height uint64, round int32) {
	timeoutPropose := config.Config.TimeoutPropose
	if h.Height != height || round < h.Round || (h.Round == round && types.RoundStepPropose <= h.Step) {
		ConsLog.Warningf(LOGTABLE_CONS, "Invalid args. Current step: %v/%v/%v/%v/%v", height, round, h.Height, h.Round, h.Step)
		return
	}

	h.timeState.ResetConsumeTime(h)
	h.timeState.SetEnterProposalStart()
	h.TryProposalTimes = 0

	ConsLog.Infof(LOGTABLE_CONS, "enterPropose(%v/%v). Current: %v/%v/%v", height, round, h.Height, h.Round, h.Step)

	defer func() {
		// Done enterPropose:
		h.updateRoundStep(round, types.RoundStepPropose)
		h.newStep()

		// If we have the whole proposal + POL, then goto Prevote now.
		// else, we'll enterPrevote when the rest of the proposal is received (in AddProposalBlockPart),
		// or else after timeoutPropose
		if h.isProposalComplete() {
			h.timeState.EndConsumeTime(h)
			h.enterPrevote(height, h.Round)
		}
	}()

	h.scheduleTimeout(h.AddTimeOut(timeoutPropose), height, round, types.RoundStepPropose)
	// If we don't get the proposal and all block parts quick enough, enterPrevote

	address := h.digestAddr
	if address == nil {
		ConsLog.Infof(LOGTABLE_CONS, "This node is not a validator")
		return
	}

	// if not a validator, we're done
	if !h.Validators.HasAddress(address) {
		ConsLog.Infof(LOGTABLE_CONS, "This node is not a validator %s", address)
		return
	}

	selected, val := h.isProposerFunc(h, height, round)
	if val == nil {
		ConsLog.Warningf(LOGTABLE_CONS, "#(%d-%d-%v) no proposer selected return", h.Height, h.Round, h.Step)
		return
	}

	if selected {
		ConsLog.Infof(LOGTABLE_CONS, "#(%d-%d-%v) enterPropose Our turn to propose,require:%v,i am %v", h.Height, h.Round, h.Step, val.Address, h.digestAddr)
		h.decideProposal(height, round)
	} else {
		//	h.scheduleTimeout(time.Second*50, height, round, types.RoundStepPrecommitWait)
		if val == nil {
			ConsLog.Errorf(LOGTABLE_CONS, "#(%d-%d-%v) enterPropose  Not our turn to propose get require nil ,i am %v", h.Height, h.Round, h.Step, h.digestAddr)
			return
		}
		ConsLog.Infof(LOGTABLE_CONS, "#(%d-%d-%v) enterPropose  Not our turn to propose,require:%v,i am %v", h.Height, h.Round, h.Step, val.Address, h.digestAddr)
	}
}

// Returns true if the proposal block is complete &&
// (if POLRound was proposed, we have +2/3 prevotes from there)
func (h *TDMMsgHandler) isProposalComplete() bool {
	if h.Proposal == nil || h.ProposalBlock == nil {
		return false
	}
	// we have the proposal. if there's a POLRound,
	// make sure we have the prevotes from it too
	if h.Proposal.POLRound < 0 {
		return true
	}
	// if this is false the proposer is lying or we haven't received the POL yet
	return h.Votes.Prevotes(h.Proposal.POLRound).HasTwoThirdsMajority()

}

func (h *TDMMsgHandler) isProposer() bool {
	ConsLog.Infof(LOGTABLE_CONS, "(%v,%v) (findProposer) bgId=%v,validators=%v", h.Height, h.Round, h.Validators.BgID, h.Validators)
	return bytes.Equal(h.Validators.GetProposerByMod(h.Height, h.Round).Address, h.digestAddr)
}

func (h *TDMMsgHandler) isProposerByVRF(height uint64, round int32) (selected bool, val *types.Validator) {

	ConsLog.Debugf(LOGTABLE_CONS, "#(%v-%v) calc proposer preVrfValue %x, vals %s", height, round, h.LastCommitState.PrevVRFValue, h.Validators)
	proposer := vrf.CalcProposerByVRF(h.LastCommitState.PrevVRFValue, h.Validators, height, round)
	return bytes.Equal(h.digestAddr, proposer.Address), proposer
}

func (h *TDMMsgHandler) decideProposal(height uint64, round int32) {
	var block *types.Block
	var blockParts *types.PartSet

	if h.LockedBlock != nil {
		ConsLog.Infof(LOGTABLE_CONS, "(%v,%v) LockedBlock is not null use LockedBlock=%v", h.Height, h.Round, h.LockedBlock)
		block, blockParts = h.LockedBlock, h.LockedBlockParts
	} else {
		block, blockParts = h.geneProposalBlkFunc(h)
		if block == nil {
			return
		}
	}

	polRound, polBlockID := h.Votes.POLInfo()
	proposal := types.NewProposal(height, round, blockParts.Header(), polRound, polBlockID)

	internalProposalMsg, err := h.buildProposalMsgFromType(proposal)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "build proposal msg err %v", err)
		return
	}
	internalProposalMsg.Flag = FlagZero
	h.sendInternalMessage(internalProposalMsg)

	for i := 0; i < blockParts.Total(); i++ {
		part := blockParts.GetPart(i)
		blkPartMsg, err := h.buildBlockPartMsgFromType(part)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "build part msg err %v", err)
			return
		}

		blkPartMsg.Flag = FlagThree
		h.sendInternalMessage(blkPartMsg)
	}

	m, _ := json.Marshal(block.Header)
	ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) (decideProposal) height=%v,round=%v,block=%v", h.Height, h.Round, height, round, string(m))
}

func (h *TDMMsgHandler) createProposalBlock() (block *types.Block, blockParts *types.PartSet) {

	var commit *types.Commit
	if h.Height == 1 {
		// We're creating a proposal for the first block.
		// The commit is empty, but not nil.
		commit = &types.Commit{}
	} else if h.LastCommit.HasTwoThirdsMajority() {
		commit = h.LastCommit.MakeCommit()
	} else {
		ConsLog.Errorf(LOGTABLE_CONS, "enterPropose: Cannot propose anything: No commit for the previous block.")
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

	_, val := h.Validators.GetByAddress(h.digestAddr)

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

func (h *TDMMsgHandler) ComputeNewVRF() (VRFValue, VRFProof []byte, err error) {
	VRFBlkData := &vrf.VRFBlkData{
		BlockNum: h.Height,
		PrevVrf:  h.LastCommitState.PrevVRFValue,
	}

	sk := msp.GetLocalPrivKey()

	VRFValue, VRFProof, err = vrf.ComputeVRF4Blk(sk, VRFBlkData, msp.GetAlgType())
	if err != nil {

		return nil, nil, fmt.Errorf("(%v-%v) (proposal) ComputeVRF4Blk err %v", h.Height, h.Round, err)
	}

	return VRFValue, VRFProof, nil
}

func (h *TDMMsgHandler) buildBlockPartMsgFromType(part *types.Part) (*cmn.PeerMessage, error) {
	blkPartMsg := &cmn.BlockPartMessage{
		Height: h.Height,
		Round:  h.Round,
		Part: &cmn.Part{
			Bytes: part.Bytes,
			Index: uint32(part.Index),
			Proof: part.Proof.Aunts,
		},
	}

	blkPartData, err := json.Marshal(blkPartMsg)
	if err != nil {
		return nil, err
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: blkPartData,
		Type:    cmn.TDMType_MsgBlockPart,
	}

	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		return nil, err
	}

	tdmData, err := json.Marshal(tdmMsgSign)

	if err != nil {
		return nil, err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return nil, err
	}

	peerMsg := &cmn.PeerMessage{
		Sender:      h.ID,
		Msg:         conData,
		IsBroadCast: false,
	}

	return peerMsg, nil
}

func (h *TDMMsgHandler) buildProposalMsgFromType(proposal *types.Proposal) (*cmn.PeerMessage, error) {

	blkPartsHeader := &cmn.PartSetHeader{
		Total: uint32(proposal.BlockPartsHeader.Total),
		Hash:  proposal.BlockPartsHeader.Hash,
	}

	polBlkID := &cmn.BlockID{
		Hash: proposal.POLBlockID.Hash,
		PartsHeader: &cmn.PartSetHeader{
			Hash:  proposal.POLBlockID.PartsHeader.Hash,
			Total: uint32(proposal.POLBlockID.PartsHeader.Total),
		},
	}

	proposalMsg := &cmn.ProposalMessage{
		Height:           proposal.Height,
		Round:            proposal.Round,
		BlockPartsHeader: blkPartsHeader,
		PoLBlockID:       polBlkID,
		PoLRound:         proposal.POLRound,
		Timestamp:        util.GetCurrentMillisecond(),
	}

	proposalData, err := json.Marshal(proposalMsg)
	if err != nil {
		return nil, err
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: proposalData,
		Type:    cmn.TDMType_MsgProposal,
	}

	//add sign
	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		return nil, err
	}

	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		return nil, err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return nil, err
	}

	peerMsg := &cmn.PeerMessage{
		Sender:      h.ID,
		Msg:         conData,
		IsBroadCast: false,
	}

	return peerMsg, nil
}
