package msgHandler

import (
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/state"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	"time"
)

func (h *TDMMsgHandler) rollBack(begin, end uint64) error {
	return ledger.RollBackLedger(begin)
}

func (h *TDMMsgHandler) reloadLastBlock() error {
	height, err := ledger.GetBlockHeight()
	if err != nil {
		return err
	}
	blk, err := ledger.GetBlock(height - 1)
	if err != nil {
		return err
	}
	if blk == nil {
		return fmt.Errorf("blk %d is nil", height-1)
	}
	ConsLog.Infof(LOGTABLE_CONS, "the last blk %d", blk.Header.BlockNum)
	status, err := h.LoadLastCommitStateFromBlkAndMem(blk)
	if err != nil {
		return err
	}
	ConsLog.Infof(LOGTABLE_CONS, "the last blk  status height %d", status.LastBlockNum)
	hash, err := ledger.CalcBlockHash(blk)
	if err != nil {
		return err
	}
	status.PreviousHash = hash

	err = h.reloadState(*status)
	if err != nil {
		return err
	}
	ConsLog.Infof(LOGTABLE_CONS, "after update status height %d", h.Height)
	err = h.reconstructLastCommit(*status, blk)
	if err != nil {
		return err
	}
	return nil
}

func (h *TDMMsgHandler) reloadState(state state.State) error {

	validators := state.Validators
	lastPrecommits := (*types.VoteSet)(nil)
	if h.CommitRound > -1 && h.Votes != nil {
		if !h.Votes.Precommits(h.CommitRound).HasTwoThirdsMajority() {
		}
		lastPrecommits = h.Votes.Precommits(h.CommitRound)
	}

	height := state.LastBlockNum + 1

	h.updateNextBlkNum(height)
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
	h.Votes = types.NewBlkNumVoteSet(height, validators)
	h.CommitRound = -1
	h.LastCommit = lastPrecommits
	h.LastValidators = state.LastValidators

	h.LastCommitState = state

	// Finally, broadcast RoundState
	if !h.isSyncStatus.Get() {
		h.newStep()
	}

	return nil
}
