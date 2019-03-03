package msgHandler

import (
	"encoding/json"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	"time"
)

//-----------------------------------------------------------------------------
// State functions 进入条件
// Used internally by handleTimeout and handleMsg to make state transitions

// Enter: `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
// 	or, if SkipTimeout==true, after receiving all precommits from (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
func (h *TDMMsgHandler) enterNewRound(height uint64, round int32) {

	if h.Height != height || round < h.Round || (h.Round == round && h.Step != types.RoundStepNewHeight) {
		ConsLog.Warningf(LOGTABLE_CONS, "enterNewRound(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, h.Height, h.Round, h.Step)
		return
	}
	//各流程时间重置
	h.timeState.SetEnterNewRoundStart()
	h.timeState.ResetConsumeTime(h)

	ConsLog.Infof(LOGTABLE_CONS, "enterNewRound(%v/%v) --> Current: %v/%v/%v", height, round, h.Height, h.Round, h.Step)

	validators := h.Validators

	if h.Round < round {
		ConsLog.Warningf(LOGTABLE_CONS, "enterNewRound(%v/%v) --> Current h.Round %v calc accum times %v", height, round, h.Round, round-h.Round)
		validators = validators.Copy()
		validators.IncrementAccum(round - h.Round)
	}

	h.updateRoundStep(round, types.RoundStepNewRound)
	h.Validators = validators

	if round != 0 {
		h.Proposal = nil
		h.ProposalBlock = nil
		h.ProposalBlockParts = nil
	}

	// also track next round (round+1) to allow round-skipping
	h.Votes.SetRound(round + 1)

	h.dealNewRound()
}

func (h *TDMMsgHandler) proposalHeartbeat(height uint64, round int32) {
	counter := 0
	addr := h.digestAddr
	valIndex, _ := h.Validators.GetByAddress(addr)

	for {
		rs := h.GetRoundState()
		// if we've already moved on, no need to send more heartbeats
		if rs.Step > types.RoundStepNewRound || rs.Round > round || rs.Height > height {
			return
		}

		hearbeatMsg := &cmn.ProposalHeartbeatMessage{
			Height:           h.Height,
			Round:            h.Round,
			Sequence:         uint32(counter),
			ValidatorAddress: addr,
			ValidatorIndex:   uint32(valIndex),
		}

		h.broadcastProposalHeartBeat(hearbeatMsg)

		counter++
		time.Sleep(proposalHeartbeatIntervalSeconds * time.Second)
	}
}

func (h *TDMMsgHandler) broadcastProposalHeartBeat(proposalHeartBeatMsg *cmn.ProposalHeartbeatMessage) error {

	phbData, err := json.Marshal(proposalHeartBeatMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "marshal phbMsg err %v", err)
		return err
	}

	tdmMsg := &cmn.TDMMessage{
		Type:    cmn.TDMType_MsgProposalHeartBeat,
		Payload: phbData,
	}

	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		return err
	}

	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "marshal tdmMsg err %v", err)
		return err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return err
	}

	pm := cmn.PeerMessage{}
	pm.Msg = conData
	h.BroadcastMsgToAll(&pm)

	return nil
}
