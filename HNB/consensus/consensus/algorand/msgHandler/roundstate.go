package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/state"
	"HNB/consensus/algorand/types"
	"HNB/consensus/consensusManager/comm/consensusType"
	"HNB/p2pNetwork"
	"HNB/p2pNetwork/message/reqMsg"
	"encoding/json"
	"fmt"
	"time"
)

// GetRoundState returns a shallow copy of the internal consensus state.
func (h *TDMMsgHandler) GetRoundState() *types.RoundState {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	rs := h.RoundState // copy
	return &rs
}

//------------------------------------------------------------
// internal functions for managing the state
func (h *TDMMsgHandler) updateNextBlkNum(nextBlkNum uint64) {
	h.Height = nextBlkNum
}

//update round step
func (h *TDMMsgHandler) updateRoundStep(round int32, step types.RoundStepType) {
	h.Round = round
	h.Step = step
}

func (h *TDMMsgHandler) updateToState(state state.State) error {

	if h.CommitRound > -1 && 0 < h.Height && h.Height != state.LastBlockNum {
		ConsLog.Errorf(LOGTABLE_CONS, "updateToState() expected state nextBlkNum of %v != %v",
			h.Height, state.LastBlockNum)

		return fmt.Errorf("updateToState() expected state nextBlkNum of %v != %v",
			h.Height, state.LastBlockNum)
	}

	if !h.LastCommitState.IsEmpty() && h.LastCommitState.LastBlockNum+1 != h.Height {
		ConsLog.Errorf(LOGTABLE_CONS, "Inconsistent cs.state.LastBlockNum+1 %v vs cs.BlockNum %v",
			h.LastCommitState.LastBlockNum+1, h.Height)

		return fmt.Errorf("inconsistent cs.state.LastBlockNum+1 %v vs cs.BlockNum %v",
			h.LastCommitState.LastBlockNum+1, h.Height)
	}

	if !h.LastCommitState.IsEmpty() && (state.LastBlockNum <= h.LastCommitState.LastBlockNum) {
		ConsLog.Infof(LOGTABLE_CONS, "Ignoring updateToState()", "newHeight", state.LastBlockNum+1, "oldHeight", h.LastCommitState.LastBlockNum+1)
		return nil
	}

	validators := state.Validators
	lastPrecommits := (*types.VoteSet)(nil)

	if h.CommitRound > -1 && h.Votes != nil {
		if !h.Votes.Precommits(h.CommitRound).HasTwoThirdsMajority() {
			cmn.PanicSanity("updateToState(state) called but last Precommit round didn't have +2/3")
		}
		lastPrecommits = h.Votes.Precommits(h.CommitRound)
	}

	// 期望下一次块号
	nextBlkNum := state.LastBlockNum + 1

	//更新期望的块号
	h.updateNextBlkNum(nextBlkNum)
	//初始化轮次和状态
	h.updateRoundStep(0, types.RoundStepNewHeight)

	if h.CommitTime.IsZero() {
		h.StartTime = h.Commit(time.Now())
	} else {
		h.StartTime = h.Commit(h.CommitTime)
	}

	h.Validators = validators
	h.Proposal = nil
	h.ProposalBlock = nil
	h.ProposalBlockParts = nil
	h.Votes = types.NewBlkNumVoteSet(nextBlkNum, validators)

	h.CommitRound = -1
	h.LastCommit = lastPrecommits
	h.LastValidators = state.LastValidators
	h.LastCommitState = state

	if !h.isSyncStatus.Get() {
		h.newStep()
	}

	return nil
}

func (h *TDMMsgHandler) getcurrentPrecommits() *types.VoteSet {
	lastPrecommits := (*types.VoteSet)(nil)
	if h.CommitRound > -1 && h.Votes != nil {
		if !h.Votes.Precommits(h.CommitRound).HasTwoThirdsMajority() {
			cmn.PanicSanity("updateToState(state) called but last Precommit round didn't have +2/3")
		}
		lastPrecommits = h.Votes.Precommits(h.CommitRound)
	}
	return lastPrecommits
}

func (h *TDMMsgHandler) newStep() {
	return
}

func (h *TDMMsgHandler) broadcastNewRoundStep(nrsMsg *cmn.NewRoundStepMessage) {
	nrsData, err := json.Marshal(nrsMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "marshal nrsMsg err %v", err)
		return
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: nrsData,
		Type:    cmn.TDMType_MsgNewRoundStep,
	}
	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "sign tdmMsg err %v", err)
		return
	}

	//tdmData, err := proto.Marshal(tdmMsg)
	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "marshal tdmMsg err %v", err)
		return
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return
	}
	p2pNetwork.Xmit(reqMsg.NewConsMsg(conData), true)
}

func (h *TDMMsgHandler) makeNewRoundStepMessage(rs types.RoundState) *cmn.NewRoundStepMessage {
	return &cmn.NewRoundStepMessage{
		Height:                rs.Height,
		Round:                 rs.Round,
		Step:                  uint32(rs.Step),
		SecondsSinceStartTime: uint32(time.Since(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommit.Round(),
	}
}
