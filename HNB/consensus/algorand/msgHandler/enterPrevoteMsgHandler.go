package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/types"
	"time"
	"strconv"
	"HNB/config"
	"HNB/p2pNetwork"
	"encoding/json"
)

func (h *TDMMsgHandler) enterPrevote(height uint64, round int32) {
	timeoutPrevote := config.Config.TimeoutPrevote
	ConsLog.Debugf(LOGTABLE_CONS, "height:(%v,%v), round:(%v,%v)," +
		"step:(%v,%v)", h.Height, height, h.Round, round,
		types.RoundStepPrevote, h.Step)

	if h.Height != height || round < h.Round || (h.Round == round && types.RoundStepPrevote <= h.Step) {
		ConsLog.Warningf(LOGTABLE_CONS, cmn.Fmt("tendermint: enterPrevote(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, h.Height, h.Round, h.Step))
		return
	}
	h.timeState.ResetConsumeTime(h)
	h.timeState.SetEnterPreVoteStart()

	// Sign and broadcast vote as necessary
	// 必要时签署和广播投票
	result := h.doPrevote(height, round)
	//如果结果是1 表示提案块可能还没有收到
	if result == 1 {
		h.scheduleTimeout(h.AddTimeOut(1000), height, round, types.RoundStepPropose)
		return
	}
	defer func() {
		//h.Logger.Warning("进入defer func")
		// Done enterPrevote:
		h.updateRoundStep(round, types.RoundStepPrevote)
		h.timeState.EndConsumeTime(h)
		h.newStep()
	}()
	//设置preote时间
	h.scheduleTimeout(h.AddTimeOut(timeoutPrevote), height, round, types.RoundStepPrevote)
	// Once `applyAddVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (h *TDMMsgHandler) defaultDoPrevote(height uint64, round int32) uint64 {
	if h.ProposalBlock == nil {
		//如果优先收到预投票块
		ConsLog.Warningf(LOGTABLE_CONS, "#(%v-%v) (defaultDoPrevote) ProposalBlock is nil TryProposal,time=%v", h.Height, h.Round, h.TryProposalTimes)
		if h.TryProposalTimes < 15 {
			h.TryProposalTimes = h.TryProposalTimes + 1
			return 1
		} else {
			ConsLog.Warningf(LOGTABLE_CONS, "#(%v-%v) (defaultDoPrevote) ProposalBlock is nil quit loop,loop time=", h.Height, h.Round, config.Config.BgDemandTimeout)
		}

	}

	//如果当前存在锁定块并且本提案的块Hash与锁定块不同，而且距离锁定块的round已经超过一轮
	if h.LockedBlock != nil && !h.LockedBlock.HashesTo(h.ProposalBlock.Hash()) && (h.Round-h.LockedRound) > int32(len(h.Validators.Validators)) {
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
// 签署投票并在发布到队列
func (h *TDMMsgHandler) signAddVote(type_ byte, hash []byte, header types.PartSetHeader) *types.Vote {
	//不是共识组不用提交
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
	vote := &types.Vote{
		ValidatorAddress: types.Address(addr),
		ValidatorIndex:   int(valIndex),
		Height:           h.Height,
		Round:            h.Round,
		Timestamp:        time.Now().UTC(),
		Type:             type_,
		BlockID:          types.BlockID{hash, header},
	}
	return vote, nil
}

func (h *TDMMsgHandler) buildHashVoteMsgFromType(vote *types.Vote) (*cmn.PeerMessage, error) {
	bitarray := &cmn.BitArray{ //TODO binarray从哪里赋值

	}
	HashVoteMsg := &cmn.HasVoteMessage{
		Height:        vote.Height,
		Round:         vote.Round,
		VoteType:      cmn.VoteType(vote.Type), //TODO INDEX没有赋值
		VotesBitArray: bitarray,
	}

	votemsg, err := json.Marshal(HashVoteMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "buildHashVoteMsgFromType Marshal err %s", err.Error())
		return nil, err
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: votemsg,
		Type:    cmn.TDMType_MsgVote, //TODO 不确定该值是否正确
	}

	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "TDMType_MsgVote payload %s,%v", tdmMsg.Payload, err)
		return nil, err
	}

	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		h.Logger.Errorf(" (buildHashVoteMsgFromType) the TDMType_MsgVote payload %s,err", tdmMsg.Payload, err)
		return nil, err
	}

	netMsg := &netProto.Message{
		Type:      netProto.Message_CONSENSUS,
		Payload:   tdmData,
		ChainId:   h.ChainID,
		Timestamp: strconv.FormatUint(util.GetCurrentMillisecond(), 10),
	}

	peerMsg := &txComm.PeerMessage{
		Sender:      h.ID,
		Msg:         netMsg,
		IsBroadCast: false,
	}

	return peerMsg, nil
}
func (h *TDMMsgHandler) broadcatstVote(msg *cmn.PeerMessage) {

	p2pNetwork.Xmit(,true)
	h.Network.Broadcast(msg.Msg, netProto.PeerEndpoint_VALIDATOR)
}

func (h *TDMMsgHandler) broadcatstHashVote(msg *cmn.PeerMessage) {
	h.Network.Broadcast(msg.Msg, netProto.PeerEndpoint_VALIDATOR)
}
