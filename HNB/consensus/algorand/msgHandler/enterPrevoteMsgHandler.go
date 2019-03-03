package msgHandler

import (
	"encoding/json"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"time"
)

func (h *TDMMsgHandler) enterPrevote(height uint64, round int32) {
	timeoutPrevote := config.Config.TimeoutPrevote
	ConsLog.Debugf(LOGTABLE_CONS, "height:(%v,%v), round:(%v,%v),"+
		"step:(%v,%v)", h.Height, height, h.Round, round,
		types.RoundStepPrevote, h.Step)

	if h.Height != height || round < h.Round || (h.Round == round && types.RoundStepPrevote <= h.Step) {
		ConsLog.Warningf(LOGTABLE_CONS, cmn.Fmt("tendermint: enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, h.Height, h.Round, h.Step))
		return
	}
	h.timeState.ResetConsumeTime(h)
	h.timeState.SetEnterPreVoteStart()

	// Sign and broadcast vote as necessary
	result := h.doPrevote(height, round)
	if result == 1 {
		h.scheduleTimeout(h.AddTimeOut(1000), height, round, types.RoundStepPropose)
		return
	}
	defer func() {
		h.updateRoundStep(round, types.RoundStepPrevote)
		h.timeState.EndConsumeTime(h)
		h.newStep()
	}()
	h.scheduleTimeout(h.AddTimeOut(timeoutPrevote), height, round, types.RoundStepPrevote)
}

func (h *TDMMsgHandler) defaultDoPrevote(height uint64, round int32) uint64 {
	if h.ProposalBlock == nil {
		ConsLog.Warningf(LOGTABLE_CONS, "#(%v-%v) (defaultDoPrevote) ProposalBlock is nil TryProposal,time=%v", h.Height, h.Round, h.TryProposalTimes)
		if h.TryProposalTimes < 2 { // < 5
			h.TryProposalTimes = h.TryProposalTimes + 1
			return 1
		} else {
			ConsLog.Warningf(LOGTABLE_CONS, "#(%v-%v) (defaultDoPrevote) ProposalBlock is nil quit loop,loop time=", h.Height, h.Round, config.Config.BgDemandTimeout)
		}

	}

	if h.LockedBlock != nil && !h.LockedBlock.HashesTo(h.ProposalBlock.Hash()) && (h.Round-h.LockedRound) < int32(len(h.Validators.Validators)) {
		ConsLog.Infof(LOGTABLE_CONS, "enterPrevote: Block was locked")
		h.signAddVote(types.VoteTypePrevote, h.LockedBlock.Hash(), h.LockedBlockParts.Header())
		return 0
	}

	if h.ProposalBlock == nil {
		ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) (defaultDoPrevote) ProposalBlock is nil", h.Height, h.Round)
		h.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return 0
	}

	err := h.blockExec.ValidateBlock(h.LastCommitState, h.ProposalBlock) //TODO 是否要签名
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "enterPrevote: ProposalBlock is invalid %v", err.Error())
		h.signAddVote(types.VoteTypePrevote, nil, types.PartSetHeader{})
		return 0
	}

	ConsLog.Infof(LOGTABLE_CONS, "enterPrevote: ProposalBlock is valid")
	h.signAddVote(types.VoteTypePrevote, h.ProposalBlock.Hash(), h.ProposalBlockParts.Header())
	return 0
}

//Enter: any +2/3 prevotes at next round.
func (h *TDMMsgHandler) enterPrevoteWait(height uint64, round int32) {
	timeoutPrevoteWait := config.Config.TimeoutPrevoteWait
	if h.Height != height || round < h.Round || (h.Round == round && types.RoundStepPrevoteWait <= h.Step) {
		ConsLog.Warningf(LOGTABLE_CONS, cmn.Fmt("enterPrevoteWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, h.Height, h.Round, h.Step))
		return
	}

	ConsLog.Infof(LOGTABLE_CONS, "enterPrevoteWait(%v/%v). Current: %v/%v/%v", height, round, h.Height, h.Round, h.Step)
	defer func() {
		// Done enterPrevoteWait:
		h.updateRoundStep(round, types.RoundStepPrevoteWait)
		h.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	h.scheduleTimeout(h.AddTimeOut(timeoutPrevoteWait), height, round, types.RoundStepPrevoteWait)
}

// sign the vote and publish on internalMsgQueue
func (h *TDMMsgHandler) signAddVote(type_ byte, hash []byte, header types.PartSetHeader) *types.Vote {
	if h.digestAddr == nil || !h.Validators.HasAddress(h.digestAddr) {
		ConsLog.Errorf(LOGTABLE_CONS, "#(%v-%v) (signAddVote)h.digestAddr == nil or !h.Validators.HasAddress(h.digestAddr) return nil,h.digestAddr=%v", h.Height, h.Round, h.digestAddr)
		return nil
	}
	vote, err := h.signVote(type_, hash, header) //获取提案者addr 组装成自己的一票投票
	if err == nil {
		internalVoteMsg, _ := h.buildVoteMsgFromType(vote)
		internalVoteMsg.Flag = FlagOne
		h.sendInternalMessage(internalVoteMsg) //发送消息
		return vote
	} else {
		ConsLog.Errorf(LOGTABLE_CONS, "sign vote err %v", err.Error())
	}

	return nil
}

func (h *TDMMsgHandler) signVote(type_ byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	addr := h.digestAddr
	valIndex, _ := h.Validators.GetByAddress(addr)
	timeSeconds := time.Unix(time.Now().UTC().Unix(), 0).UTC()
	vote := &types.Vote{
		ValidatorAddress: types.Address(addr),
		ValidatorIndex:   int(valIndex),
		Height:           h.Height,
		Round:            h.Round,
		Timestamp:        timeSeconds,
		Type:             type_,
		BlockID:          types.BlockID{hash, header},
	}

	vote, err := h.mySignVote(vote)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "sign vote err %v", err)
		return nil, fmt.Errorf("sign vote err %v", err)
	}

	return vote, nil
}

func (h *TDMMsgHandler) buildHashVoteMsgFromType(vote *types.Vote) (*cmn.PeerMessage, error) {
	bitarray := &cmn.BitArray{}
	HashVoteMsg := &cmn.HasVoteMessage{
		Height:        vote.Height,
		Round:         vote.Round,
		VoteType:      cmn.VoteType(vote.Type),
		VotesBitArray: bitarray,
	}

	votemsg, err := json.Marshal(HashVoteMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "buildHashVoteMsgFromType Marshal err %s", err.Error())
		return nil, err
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: votemsg,
		Type:    cmn.TDMType_MsgVote,
	}

	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "TDMType_MsgVote payload %s,%v", tdmMsg.Payload, err)
		return nil, err
	}

	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "TDMType_MsgVote payload %s,%v", tdmMsg.Payload, err)
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

func (h *TDMMsgHandler) broadcatstVote(msg *cmn.PeerMessage) {
	m, _ := json.Marshal(msg)
	p2pNetwork.Xmit(reqMsg.NewConsMsg(m), true)
}

func (h *TDMMsgHandler) broadcatstHashVote(msg *cmn.PeerMessage) {
	m, _ := json.Marshal(msg)
	p2pNetwork.Xmit(reqMsg.NewConsMsg(m), true)
}
