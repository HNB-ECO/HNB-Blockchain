package msgHandler

import (
	"fmt"
	"HNB/p2pNetwork/message/reqMsg"
	"time"
	"HNB/consensus/algorand/types"
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/state"
	"encoding/json"
	"HNB/p2pNetwork"
)

func (h *TDMMsgHandler) GetRoundState() *types.RoundState {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	rs := h.RoundState
	return &rs
}

func (h *TDMMsgHandler) updateNextBlkNum(nextBlkNum uint64) {
	h.Height = nextBlkNum
}

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

	nextBlkNum := state.LastBlockNum + 1
	h.updateNextBlkNum(nextBlkNum)
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
