package msgHandler

import (
	"encoding/json"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/state"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
	"time"
)

func (h *TDMMsgHandler) enterPrecommit(height uint64, round int32) {
	timeoutPrecommit := config.Config.TimeoutPrecommit
	ConsLog.Infof(LOGTABLE_CONS, cmn.Fmt("enterPrecommit(%v/%v). Current: %v/%v/%v", height, round, h.Height, h.Round, h.Step))
	if h.Height != height || round < h.Round || (h.Round == round && types.RoundStepPrecommit <= h.Step) {
		ConsLog.Debugf(LOGTABLE_CONS, cmn.Fmt("enterPrecommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, h.Height, h.Round, h.Step))
		return
	}
	h.timeState.ResetConsumeTime(h)

	defer func() {
		// Done enterPrecommit:
		h.updateRoundStep(round, types.RoundStepPrecommit)
		h.scheduleTimeout(h.AddTimeOut(timeoutPrecommit), height, round, types.RoundStepPrecommit)
		h.timeState.EndConsumeTime(h)
		h.newStep()
	}()

	blockID, ok := h.Votes.Prevotes(round).TwoThirdsMajority()
	if !ok {
		ConsLog.Infof(LOGTABLE_CONS, "(%v,%v)enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.", h.Height, h.Round)
		h.applyAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}

	// the latest POLRound should be this round
	polRound, _ := h.Votes.POLInfo()

	if polRound < round {
		//cmn.PanicSanity(cmn.Fmt("This POLRound should be %v but got %", round, polRound))
		ConsLog.Errorf(LOGTABLE_CONS, cmn.Fmt("This POLRound should be %v but got %", round, polRound))
	}

	if len(blockID.Hash) == 0 {
		ConsLog.Infof(LOGTABLE_CONS, "(%v,%v) enterPrecommit: +2/3 prevoted for nil.", h.Height, h.Round)
		h.applyAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})
		return
	}
	// If +2/3 prevoted for proposal block, stage and precommit it
	if h.ProposalBlock.HashesTo(blockID.Hash) {
		ConsLog.Infof(LOGTABLE_CONS, "enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockID.Hash)
		// Validate the block.
		h.LockedRound = round
		h.LockedBlock = h.ProposalBlock
		h.LockedBlockParts = h.ProposalBlockParts
		h.applyAddVote(types.VoteTypePrecommit, blockID.Hash, blockID.PartsHeader)
		return
	}
	h.LockedRound = 0
	h.LockedBlock = nil
	h.LockedBlockParts = nil
	h.applyAddVote(types.VoteTypePrecommit, nil, types.PartSetHeader{})

}

// Enter: any +2/3 precommits for next round.
func (h *TDMMsgHandler) enterPrecommitWait(height uint64, round int32) {
	timeoutPrecommitWait := config.Config.TimeoutPrecommitWait

	defer func() {
		// Done enterPrecommitWait:
		h.updateRoundStep(round, types.RoundStepPrecommitWait)
		h.scheduleTimeout(h.AddTimeOut(timeoutPrecommitWait), height, round, types.RoundStepPrecommitWait)
		h.newStep()
	}()

	if h.Height != height || round < h.Round || (h.Round == round && types.RoundStepPrecommitWait <= h.Step) {
		ConsLog.Debugf(LOGTABLE_CONS, cmn.Fmt("enterPrecommitWait(%v/%v): Invalid args. Current step: %v/%v/%v", height, round, h.Height, h.Round, h.Step))
		return
	}

	ConsLog.Infof(LOGTABLE_CONS, cmn.Fmt("enterPrecommitWait(%v/%v). Current: %v/%v/%v", height, round, h.Height, h.Round, h.Step))
	if !h.Votes.Precommits(round).HasTwoThirdsAny() {
		//cmn.PanicSanity(cmn.Fmt("enterPrecommitWait(%v/%v), but Precommits does not have any +2/3 votes", height, round))
		return
	}
	// Wait for some more precommits; enterNewRound
	//h.scheduleTimeout(5*time.Second, height, round, types.RoundStepPrecommitWait)

}

// Enter: +2/3 precommits for block
func (h *TDMMsgHandler) enterCommit(height uint64, commitRound int32) {
	if h.Height != height || types.RoundStepCommit <= h.Step {
		ConsLog.Debugf(LOGTABLE_CONS, cmn.Fmt("enterCommit(%v/%v): Invalid args. Current step: %v/%v/%v", height, commitRound, h.Height, h.Round, h.Step))
		return
	}
	h.timeState.ResetConsumeTime(h)
	h.timeState.SetEnterPreCommitStart()
	ConsLog.Infof(LOGTABLE_CONS, cmn.Fmt("enterCommit(%v/%v). Current: %v/%v/%v", height, commitRound, h.Height, h.Round, h.Step))

	defer func() {
		// Done enterCommit:
		// keep cs.Round the same, commitRound points to the right Precommits set.
		h.updateRoundStep(h.Round, types.RoundStepCommit)
		h.CommitRound = commitRound
		h.CommitTime = time.Now()
		h.newStep()

		// Maybe finalize immediately.
		h.tryFinalizeCommit(height)
	}()

	blockID, ok := h.Votes.Precommits(commitRound).TwoThirdsMajority()
	if !ok {
		cmn.PanicSanity("RunActionCommit() expects +2/3 precommits")
	}

	// If we don't have the block being committed, set up to get it.
	if !h.ProposalBlock.HashesTo(blockID.Hash) {
		if !h.ProposalBlockParts.HasHeader(blockID.PartsHeader) {
			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			h.ProposalBlock = nil
			h.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartsHeader)
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (h *TDMMsgHandler) tryFinalizeCommit(height uint64) {
	if h.Height != height {
		cmn.PanicSanity(cmn.Fmt("tryFinalizeCommit() cs.BlockNum: %v vs height: %v", h.Height, height))
	}

	blockID, ok := h.Votes.Precommits(h.CommitRound).TwoThirdsMajority()
	if !ok || len(blockID.Hash) == 0 {
		ConsLog.Errorf(LOGTABLE_CONS, "Attempt to finalize failed. There was no +2/3 majority, or +2/3 was for <nil>.", "height", height)
		h.enterNewRound(h.Height, h.Round+1)
	}
	if !h.ProposalBlock.HashesTo(blockID.Hash) {
		ConsLog.Warningf(LOGTABLE_CONS, "Attempt to finalize failed. We don't have the commit block.", "height", height, "proposal-block", h.ProposalBlock.Hash(), "commit-block", blockID.Hash, "Lockedblock", h.LockedBlock.Hash())
		if h.LockedBlock.HashesTo(blockID.Hash) {
			h.ProposalBlock = h.LockedBlock
			h.ProposalBlockParts = h.ProposalBlockParts
		} else {
			ConsLog.Errorf(LOGTABLE_CONS, "Attempt to finalize failed. We don't have the commit block.", "height", height, "proposal-block", h.ProposalBlock.Hash(), "commit-block", blockID.Hash)
			h.enterNewRound(h.Height, h.Round+1)
		}
	}

	h.finalizeCommit(height)
}

// Increment height and goto cstypes.RoundStepNewHeight
func (h *TDMMsgHandler) finalizeCommit(height uint64) {
	if h.Height != height || h.Step != types.RoundStepCommit {
		ConsLog.Debugf(LOGTABLE_CONS, cmn.Fmt("finalizeCommit(%v): Invalid args.   ", height, h.Height, h.Round, h.Step))
		return
	}

	blockID, ok := h.Votes.Precommits(h.CommitRound).TwoThirdsMajority()
	block, blockParts := h.ProposalBlock, h.ProposalBlockParts

	if !ok {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, commit does not have two thirds majority"))
	}
	if !blockParts.HasHeader(blockID.PartsHeader) {
		cmn.PanicSanity(cmn.Fmt("Expected ProposalBlockParts header to be commit header"))
	}
	if !block.HashesTo(blockID.Hash) {
		cmn.PanicSanity(cmn.Fmt("Cannot finalizeCommit, ProposalBlock does not hash to commit hash"))
	}
	//if err := h.blockExec.ValidateBlock(cs.state, block); err != nil {
	//	cmn.PanicConsensus(cmn.Fmt("+2/3 committed an invalid block: %v", err))
	//}

	ConsLog.Infof(LOGTABLE_CONS, "(commit blk) height %d txs num %d hash %v", block.BlockNum, block.NumTxs, block.Hash())
	ConsLog.Infof(LOGTABLE_CONS, "(commit blk) blk %v", block.Header)

	// Save to blockStore.
	if h.getHeightFromLedger() < block.BlockNum {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		ConsLog.Infof(LOGTABLE_CONS, "h.getHeightFromLedger() < block.BlockNum saveBlock")
	} else {
		// Happens during replay if we already saved the block but didn't commit
		ConsLog.Infof(LOGTABLE_CONS, "Calling finalizeCommit on already stored block", "height", block.BlockNum)
	}

	//fail.Fail()

	// Create a copy of the state for staging
	// and an event cache for txs
	stateCopy := h.LastCommitState.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE: the block.AppHash wont reflect these txs until the next block
	var err error

	ConsLog.Infof(LOGTABLE_CONS, "before lastCommit: height %d valHash %v vals %v",
		block.BlockNum, stateCopy.Validators.Hash(), stateCopy.Validators)
	stateCopy, err = h.ApplyBlock(stateCopy, types.BlockID{block.Hash(), blockParts.Header()}, block)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "Error on ApplyBlock. Did the application crash? Please restart tendermint", "err", err)
		h.enterNewRound(height, h.Round+1)
		return
	}

	ConsLog.Infof(LOGTABLE_CONS, "after lastCommit: height %d valHash %v vals %v",
		block.BlockNum, stateCopy.Validators.Hash(), stateCopy.Validators)
	h.timeState.setEnd()
	//fmt.Print(h.timeState.String())
	h.timeState.setStart()

	h.execConsSucceedFuncs(block)

	// NewHeightStep!
	h.updateToState(stateCopy)

	//
	h.execStatusUpdatedFuncs()

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	h.JustStarted = false
	if h.IsContinueConsensus {
		h.scheduleRound0(&h.RoundState)
	}

	// By here,
	// * cs.BlockNum has been increment to height+1
	// * cs.Step is now cstypes.RoundStepNewHeight
	// * cs.StartTime is set to when we will start round0.
}

func (h *TDMMsgHandler) ApplyBlock(s state.State, blockID types.BlockID, block *types.Block) (state.State, error) {

	currentPrecommits := h.getcurrentPrecommits()
	block.CurrentCommit = currentPrecommits.MakeCommit()

	if err := h.blockExec.ValidateBlock(s, block); err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(%v,%v) ValidateBlock BlockFail,block=%v,err=%v", h.Height, h.Round, block, err)
		return s, err
	}

	h.timeState.SetBeforeWriteBlock()
	sSave, err := h.saveBlock(block, &s)
	h.timeState.SetAfterWriteBlock()
	if err != nil {
		return s, err
	}

	s, err = h.updateState(*sSave, blockID, block.Header)
	if err != nil {
		return s, fmt.Errorf("Commit failed for application: %v", err)
	}

	stepOld := h.Step
	h.Step = types.RoundStepCommit
	h.timeState.EndConsumeTime(h)
	h.Step = stepOld
	ConsLog.Infof(LOGTABLE_CONS, "newRoundEnd:%v,\t preposeEnd:%v,\t prevoteEnd：%v,\t precommitEnd:%v,\t commitEnd:%v", h.timeState.newRoundEnd, h.timeState.preposeEnd, h.timeState.prevoteEnd, h.timeState.precommitEnd, h.timeState.commitEnd)

	//unlock LockedBlock
	if h.LockedBlock != nil {
		ConsLog.Infof(LOGTABLE_CONS, "(%v,%v)unlock LockedBlock", h.Height, h.Round)
		h.LockedRound = 0
		h.LockedBlock = nil
		h.LockedBlockParts = nil
	}

	ConsLog.Infof(LOGTABLE_CONS, h.PrintTDMBlockInfo(block))
	return s, nil
}

func (h *TDMMsgHandler) execConsSucceedFuncs(blk *types.Block) {
	for k, consSucceedFunc := range h.consSucceedFuncChain {
		ConsLog.Debugf(LOGTABLE_CONS, "#(%d-%d) (consSucceedFunc) busi %s start to ", h.Height, h.Round, k)
		err := consSucceedFunc(h, blk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "#(%d-%d) (consSucceedFunc) busi %s exec fail err %v", h.Height, h.Round, k, err)
		}
	}
}

func (h *TDMMsgHandler) execStatusUpdatedFuncs() {
	for name, statusUpdatedFunc := range h.statusUpdatedFuncChain {
		ConsLog.Infof(LOGTABLE_CONS, "#(%d-%d) (statusUpdatedFunc) busi %s start to ", h.Height, h.Round, name)
		err := statusUpdatedFunc(h)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "#(%d-%d) (consSucceedFunc) busi %s exec fail err %v", h.Height, h.Round, name, err)
		}
	}
}

func (h *TDMMsgHandler) PrintTDMBlockInfo(block *types.Block) string {
	blkByte, _ := json.Marshal(block)
	blkSize := float64(len(blkByte)) / 1024

	var peerID types.Address
	if block.Validators == nil || block.Validators.Proposer == nil {
		peerID = types.Address{}
	} else {
		peerID = block.Validators.Proposer.Address
	}

	return fmt.Sprintf("hnb %d,%d,%d,%v,%v,%v",
		block.BlockNum,
		block.NumTxs,
		block.Header.Time,
		time.Unix(int64(block.Header.Time.Unix()/1000), 0).Format("2006-01-02 15:04:05 PM"),
		peerID,
		blkSize,
	)
}

func (h *TDMMsgHandler) updateState(s state.State, blockID types.BlockID, header *types.Header) (state.State, error) {
	prevValSet := s.Validators.Copy()
	nextValSet := prevValSet.Copy()
	lastHeightValsChanged := s.LastHeightValidatorsChanged

	nextValSet.IncrementAccum(1)
	nextParams := s.ConsensusParams
	lastHeightParamsChanged := s.LastHeightConsensusParamsChanged
	return state.State{
		LastBlockNum:                     header.BlockNum,
		LastBlockTotalTx:                 s.LastBlockTotalTx + header.NumTxs,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		Validators:                       nextValSet,
		LastValidators:                   s.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
		PreviousHash:                     s.PreviousHash,
		PrevVRFValue:                     header.BlkVRFValue,
		PrevVRFProof:                     header.BlkVRFProof,
	}, nil

}

func (h *TDMMsgHandler) applyAddVote(type_ byte, hash []byte, header types.PartSetHeader) *types.Vote {
	vi, _ := h.Validators.GetByAddress(h.digestAddr)
	timeSeconds := time.Unix(time.Now().UTC().Unix(), 0).UTC()

	vote := &types.Vote{
		ValidatorAddress: types.Address(h.digestAddr),
		ValidatorIndex:   vi,
		Height:           h.Height,
		Round:            h.Round,
		Timestamp:        timeSeconds,
		Type:             type_,
		BlockID:          types.BlockID{Hash: hash, PartsHeader: header},
	}

	vote, err := h.mySignVote(vote)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "sign vote err %v", err)
		return nil
	}

	msg, _ := h.buildVoteMsgFromType(vote)
	msg.Flag = FlagTwo

	h.sendInternalMessage(msg)
	return vote
}

func (h *TDMMsgHandler) mySignVote(vote *types.Vote) (*types.Vote, error) {
	voteBytes, err := json.Marshal(vote)
	if err != nil {
		return nil, err
	}

	signature, err := msp.Sign(voteBytes)
	if err != nil {
		return nil, err
	}

	vote.Signature = util.HexBytes(signature)
	ConsLog.Debugf(LOGTABLE_CONS, "signMsg: %v, signature: %v, addr: %v id: %v", string(voteBytes), vote.Signature, h.digestAddr, h.ID)
	return vote, nil
}

func (h *TDMMsgHandler) tryAddVote(vote *types.Vote, pubKeyID []byte) error {
	_, err := h.addVote(vote, pubKeyID)
	if err != nil {
		if err == ErrVoteHeightMismatch {
			return err
		} else {
			return ErrAddingVote
		}
	}
	return nil
}

func (h *TDMMsgHandler) addVote(vote *types.Vote, pubKeyID []byte) (added bool, err error) {
	ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) addVote(%v/%v/%v) valIndex=%v", h.Height, h.Round, vote.VoteString(), vote.Height, vote.Round, vote.ValidatorIndex)
	if vote.Round > h.Round {
		h.Votes.SetRound(vote.Round)
	}
	//本轮次结束，还收到上一轮次消息
	if vote.Height+1 == h.Height {
		ConsLog.Warningf(LOGTABLE_CONS, "A precommit for the previous height? These come in while we wait timeoutCommit")
		if !(h.Step == types.RoundStepNewHeight && vote.Type == types.VoteTypePrecommit) {
			ConsLog.Infof(LOGTABLE_CONS, "#(%d-%d) Step=%v receive vote.Type %v ignore", h.Height, h.Round, h.Step, vote.Type)
			return added, nil
		}
		//把投票信息写入上一次轮次的vote
		added, err = h.LastCommit.AddVote(vote)
		if !added {
			return added, err
		}
		ConsLog.Infof(LOGTABLE_CONS, cmn.Fmt("Added to lastPrecommits: %v", h.LastCommit.StringShort()))
		// if we can skip timeoutCommit and have all the votes now,
		if config.Config.SkipTimeoutCommit && h.LastCommit.HasAll() {
			h.enterNewRound(h.Height, 0)
		}

		return
	}

	// BlockNum mismatch is ignored.
	// Not necessarily a bad peer, but not favourable behaviour.
	if vote.Height != h.Height {
		err = ErrVoteHeightMismatch
		ConsLog.Infof(LOGTABLE_CONS, "Vote ignored and not added", "voteHeight", vote.Height, "csHeight", h.Height, "err", err)
		return
	}

	height := h.Height
	added, err = h.Votes.AddVote(vote, pubKeyID) //统计外来投票
	if !added {
		ConsLog.Warningf(LOGTABLE_CONS, "Either duplicate, or error upon h.Votes.AddByIndex() Return")
		return
	}

	switch vote.Type {
	case types.VoteTypePrevote:
		prevotes := h.Votes.Prevotes(vote.Round)
		blockID, ok := prevotes.TwoThirdsMajority()
		ConsLog.Infof(LOGTABLE_CONS, "(%v-%v) (addVote) Added to prevote (%v/%v/%v) vote=%v,prevotes=%v,TwoThirdsMajority=%v,boockID=%v", h.Height, h.Round, vote.Height, vote.Round, vote.VoteString(), vote, prevotes, ok, blockID.String())
		if h.Round < vote.Round && prevotes.HasHalfAny() {
			ConsLog.Warningf(LOGTABLE_CONS, "(%v-%v) (addVote) Added to prevote HasHalfAny Fix Round", h.Height, h.Round)
			h.enterNewRound(height, vote.Round) //Fix round
		}
		if h.Round <= vote.Round && prevotes.HasTwoThirdsAny() {
			// Round-skip over to PrevoteWait or goto Precommit.
			h.enterNewRound(height, vote.Round) // if the vote is ahead of us
			if prevotes.HasTwoThirdsMajority() {
				h.enterPrecommit(height, vote.Round)
			} else {
				h.enterPrevote(height, vote.Round) // if the vote is ahead of us
				h.enterPrevoteWait(height, vote.Round)
			}
		} else if h.Proposal != nil && 0 <= h.Proposal.POLRound && h.Proposal.POLRound == vote.Round {
			// If the proposal is now complete, enter prevote of h.Round.
			ConsLog.Warningf(LOGTABLE_CONS, "(%v-%v) (addVote) POL If the proposal is now complete, enter prevote of h.Round.", h.Height, h.Round)
			if h.isProposalComplete() {
				h.enterPrevote(height, h.Round)
			}
		}
	case types.VoteTypePrecommit:
		precommits := h.Votes.Precommits(vote.Round)
		//h.Logger.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())
		ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) (addVote) Added to precommit (%v/%v/%v) vote=%v,precommits=%v", h.Height, h.Round, vote.Height, vote.Round, vote.VoteString(), vote, precommits.StringShort())
		blockID, ok := precommits.TwoThirdsMajority()
		//是否precommit是否超过2/3
		if ok {
			if len(blockID.Hash) == 0 {
				ConsLog.Warningf(LOGTABLE_CONS, "(%v-%v) (addVote) blockID.Hash == 0  enterNewRound", h.Height, h.Round)
				h.enterNewRound(height, vote.Round+1)
			} else {
				h.enterNewRound(height, vote.Round)
				h.enterPrecommit(height, vote.Round)
				h.enterCommit(height, vote.Round)

				if config.Config.SkipTimeoutCommit && precommits.HasAll() {
					// if we have all the votes now,
					// go straight to new round (skip timeout commit)
					// h.scheduleTimeout(time.Duration(0), h.BlockNum, 0, cstypes.RoundStepNewHeight)
					h.enterNewRound(h.Height, 0)
				}

			}
		} else if h.Round <= vote.Round && precommits.HasTwoThirdsAny() {
			h.enterNewRound(height, vote.Round)
			h.enterPrecommit(height, vote.Round)
			h.enterPrecommitWait(height, vote.Round)
		}
	default:
		panic(cmn.Fmt("Unexpected vote type %X", vote.Type)) // go-wire should prevent this.
	}

	return
}

func (h *TDMMsgHandler) buildVoteMsgFromType(vote *types.Vote) (*cmn.PeerMessage, error) {
	partSet := &cmn.PartSetHeader{Total: uint32(vote.BlockID.PartsHeader.Total), Hash: vote.BlockID.PartsHeader.Hash}
	blockId := &cmn.BlockID{Hash: vote.BlockID.Hash, PartsHeader: partSet}
	vm := &cmn.VoteMessage{
		ValidatorAddress: vote.ValidatorAddress,
		ValidatorIndex:   int32(vote.ValidatorIndex),
		Height:           vote.Height,
		Round:            vote.Round,
		Timestamp:        uint64(vote.Timestamp.UTC().Unix()),
		VoteType:         cmn.VoteType(vote.Type - 1),
		BlockID:          blockId,
		Signature:        vote.Signature,
	}
	voteData, err := json.Marshal(vm)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "buildVoteMsgFromType marshal err %s", err.Error())
		return nil, err
	}
	tdmMsg := &cmn.TDMMessage{
		Type:    cmn.TDMType_MsgVote,
		Payload: voteData,
	}
	ConsLog.Debugf(LOGTABLE_CONS, "the TDMType_MsgVote payload %s", tdmMsg.Payload)
	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "the TDMType_MsgVote payload %s err: %v", tdmMsg.Payload, err)
		return nil, err
	}
	ConsLog.Debugf(LOGTABLE_CONS, "the TDMType_MsgVote payload %s", tdmMsg.Payload)
	//tdmData, err := proto.Marshal(tdmMsg)
	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "the TDMType_MsgVote payload %s err: %v", tdmMsg.Payload, err)
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

	peerMsg := &cmn.PeerMessage{Msg: conData, Sender: h.ID, IsBroadCast: false}

	return peerMsg, nil
}

func (h *TDMMsgHandler) buildTypeFromVoteMsg(tdmMsg *cmn.TDMMessage) (*types.Vote, error) {
	voteData := tdmMsg.Payload
	if voteData == nil {
		ConsLog.Infof(LOGTABLE_CONS, "(buildTypeFromVoteMsg): voteData is nil")
	}
	vm := &cmn.VoteMessage{}
	err := json.Unmarshal(voteData, vm)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(buildTypeFromVoteMsg) err", err)
		return nil, err
	}

	partHeader := types.PartSetHeader{
		Total: int(vm.BlockID.PartsHeader.Total),
		Hash:  vm.BlockID.PartsHeader.Hash,
	}
	blockId := types.BlockID{
		Hash:        vm.BlockID.Hash,
		PartsHeader: partHeader,
	}
	vote := &types.Vote{
		ValidatorAddress: vm.ValidatorAddress,
		ValidatorIndex:   int(vm.ValidatorIndex),
		Height:           vm.Height,
		Round:            vm.Round,
		Timestamp:        time.Unix(int64(vm.Timestamp), 0).UTC(),
		Type:             byte(vm.VoteType + 1),
		BlockID:          blockId,
		Signature:        util.HexBytes(vm.Signature),
	}
	return vote, nil
}
