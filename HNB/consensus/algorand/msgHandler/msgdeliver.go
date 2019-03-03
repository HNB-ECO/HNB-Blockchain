package msgHandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr"
	appComm "github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr/common"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/txpool"
	"github.com/json-iterator/go"
	"time"
)

const (
	FlagZero  = 0 //prepose
	FlagOne   = 1 // provote
	FlagTwo   = 2 //precommit
	FlagThree = 3 // blk
)

func (h *TDMMsgHandler) checkContinue() bool {
	if h.isSyncStatus.Get() == true {
		ConsLog.Infof(LOGTABLE_CONS, "peer is syncing")
		return false
	}
	if !h.IsContinueConsensus {
		ConsLog.Errorf(LOGTABLE_CONS, "%s can not ContinueConsensus", appComm.HNB)
		return false
	}
	return true
}

func (h *TDMMsgHandler) DeliverMsg() {
	h.allRoutineExitWg.Add(1)
	var peerMsg *cmn.PeerMessage
	var err error
	for {
		select {
		case height := <-h.TxsAvailable:
			if !h.checkContinue() {
				continue
			}
			h.handleTxsAvailable(height)
		case peerMsg = <-h.PeerMsgQueue:
			if !h.checkContinue() {
				continue
			}
			if peerMsg == nil {
				ConsLog.Warningf(LOGTABLE_CONS, "(msgDeliver) peerMsg is nil")
				continue
			}
			var consensusMsg = &consensusType.ConsensusMsg{}
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(peerMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(dbftMgr) unmarshal conMsg err %v", err)
				continue
			}

			tdmMsg := &cmn.TDMMessage{}
			if err := json.Unmarshal(consensusMsg.Payload, tdmMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(msgDeliver) unmarshal tdmMsg err %v", err)
				continue
			}

			err := h.Verify(tdmMsg, peerMsg.Sender)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(msgDeliver) Verify tdmMsg err %v", err)
				continue
			}

			ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) (msgDeliver) recv peerMsg <- %s type %v", h.Height, h.Round, msp.PeerIDToString(peerMsg.Sender), tdmMsg.Type)

			switch tdmMsg.Type {
			case cmn.TDMType_MsgCheckFork:
				err = h.HandleCheckFork(tdmMsg, peerMsg.PeerID)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "HandleCheckFork err %v", err.Error())
				}
			default:

				if !h.isPeerInbgGroup(peerMsg.Sender) || !h.inbgGroup() {
					ConsLog.Warningf(LOGTABLE_CONS, "(msgDeliver) recv peerMsg <- %s not in bftGroup type %v ", msp.PeerIDToString(peerMsg.Sender), tdmMsg.Type)
					continue
				}

				err = h.processOuterMsg(tdmMsg, peerMsg.Sender, peerMsg.PeerID)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "(msgDeliver) process peerMsg<-%s err %v", peerMsg.Sender, err)
				}
			}
		case internalMsg := <-h.InternalMsgQueue:
			if !h.checkContinue() {
				continue
			}

			if internalMsg == nil {
				ConsLog.Warningf(LOGTABLE_CONS, "(msgDeliver) internalMsg is nil")
				continue
			}

			h.BroadcastMsgToAll(internalMsg)

			var consensusMsg = &consensusType.ConsensusMsg{}
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err := json.Unmarshal(internalMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Warningf(LOGTABLE_CONS, "(dbftMgr) unmarshal conMsg err %v", err)
				continue
			}

			tdmMsg := &cmn.TDMMessage{}
			if err := json.Unmarshal(consensusMsg.Payload, tdmMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(msgDeliver) unmarshal tdmMsg err %v", err)
				continue
			}

			ConsLog.Infof(LOGTABLE_CONS, "(msgDeliver) recv internalMsg type %s", tdmMsg.Type)
			err = h.processInnerMsg(tdmMsg, h.ID)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(msgDeliver) process internalMsg err %v", err)
			}
		case syncData := <-h.recvSyncChan:
			if !h.IsContinueConsensus {
				ConsLog.Errorf(LOGTABLE_CONS, "%s can not ContinueConsensus", appComm.HNB)
				continue
			}
			if syncData.Version != h.syncHandler.Version {
				ConsLog.Errorf(LOGTABLE_CONS, "%s sync version not match %d != %d", CHAINID, syncData.Version, h.syncHandler.Version)
				continue
			}
			syncBlkCntPoint := h.cacheSyncBlkCount[CHAINID]
			syncBlktargetV := syncBlkCntPoint.SyncBlkCountTarget.GetCurrentIndex()
			syncBlkCntPoint.SyncBlkCountComplete.GetNextIndex()                     // 完成同步块数++1
			nowSyncBlkCnt := syncBlkCntPoint.SyncBlkCountComplete.GetCurrentIndex() // 目前已经完成同步块数
			ConsLog.Infof(LOGTABLE_CONS, "sync blk accepting syncBlktargetV<-nowSyncBlkCnt:[%d-%d],finishFlag %v v %d",
				syncBlktargetV, nowSyncBlkCnt, syncData.FinishFlag, h.syncHandler.Version)
			if syncData.FinishFlag {
				ConsLog.Infof(LOGTABLE_CONS, "sync succ")
				h.isSyncStatus.SetFalse()
				h.stopSyncTimer()
				h.syncHandler.Version++
				h.newStep()
			} else {
				block := syncData.Block
				if block == nil {
					ConsLog.Warningf(LOGTABLE_CONS, "sync err block is nil")
					h.resetSync()
					continue
				}
				blkNum := block.Header.BlockNum
				ConsLog.Infof(LOGTABLE_CONS, "recv sync blk %d FinishFlag %t", blkNum, syncData.FinishFlag)

				tdmBlk, err := types.Standard2Cons(block)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "conver err %s", err.Error())
					h.resetSync()
					continue
				}

				status, err := h.LoadLastCommitStateFromBlkAndMem(block)
				if err != nil {

					ConsLog.Errorf(LOGTABLE_CONS, "load status err %s", err)
					h.resetSync()
					continue
				}

				ConsLog.Infof(LOGTABLE_CONS, "(sync blk) %d vals change height last %d curr %d",
					blkNum, h.LastCommitState.LastHeightValidatorsChanged, status.LastHeightValidatorsChanged)
				if status.LastHeightValidatorsChanged > h.LastCommitState.LastHeightValidatorsChanged {
					ConsLog.Infof(LOGTABLE_CONS, "(sync blk) %d vals changed %v", blkNum, status.Validators)
					h.LastCommitState.Validators = status.Validators
				}

				if err := h.blockExec.ValidateBlock(h.LastCommitState, tdmBlk); err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "(sync blk) %d validate err %v", blkNum, err)
					h.resetSync()
					continue
				}

				appMgr.BlockProcess(block)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "(sync blk) %d fallBlock failed error %s", blkNum, err)
					h.resetSync()
					continue
				} else {
					hash, err := ledger.CalcBlockHash(block)
					if err != nil {
						ConsLog.Errorf(LOGTABLE_CONS, "(sync blk) %d CalcBlockHash error %s", blkNum, err)
						h.resetSync()
						continue
					}
					status.PreviousHash = hash

					if len(block.Txs) > 0 {
						txpool.DelTxs(CHAINID, block.Txs)
					}

					ConsLog.Infof(LOGTABLE_CONS, "(sync blk) %d fallBlock success", blkNum)

					ConsLog.Infof(LOGTABLE_CONS, h.PrintTDMBlockInfo(tdmBlk))
					err = h.updateToState(*status)
					if err != nil {
						ConsLog.Errorf(LOGTABLE_CONS, "update last status err %s", err)
						h.resetSync()
						continue
					}

					err = h.reconstructLastCommit(*status, block)
					if err != nil {
						ConsLog.Errorf(LOGTABLE_CONS, "update last commit err %s", err)
						h.resetSync()
						continue
					}
					h.execConsSucceedFuncs(tdmBlk)
					h.execStatusUpdatedFuncs()

					ConsLog.Infof(LOGTABLE_CONS, "(sync blk) %d lastValsChange %d", blkNum, h.LastCommitState.LastHeightValidatorsChanged)
				}
			}
		case ti := <-h.timeoutTicker.Chan():
			if !h.checkContinue() {
				continue
			}
			h.handleTimeout(ti, h.RoundState)
		case <-h.Quit():
			h.allRoutineExitWg.Done()
			ConsLog.Infof(LOGTABLE_CONS, "(msgDeliver) tdmMsgHandler consensus service stopped ")
			return
		}
	}
}

func (h *TDMMsgHandler) BroadcastMsg() {
	h.allRoutineExitWg.Add(1)
	for {
		select {
		case broadcastMsg := <-h.EventMsgQueue:
			m, _ := json.Marshal(broadcastMsg)
			p2pNetwork.Xmit(reqMsg.NewConsMsg(m), true)
		case <-h.Quit():
			h.allRoutineExitWg.Done()
			ConsLog.Infof(LOGTABLE_CONS, "(msgHandler) broadcast routine stopped ")
			return
		}
	}
}

func (h *TDMMsgHandler) getOtherVals() []*types.Validator {
	vals := make([]*types.Validator, 0)

	for _, val := range h.Validators.Validators {
		if bytes.Equal(val.Address, h.digestAddr) {
			continue
		} else {
			vals = append(vals, val)
		}
	}

	return vals
}

func (h *TDMMsgHandler) handleTxsAvailable(height uint64) {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	if h.inbgGroup() {
		h.enterPropose(height, 0)
	}
}

func (h *TDMMsgHandler) scheduleTimeout(duration time.Duration, height uint64, round int32, step types.RoundStepType) {
	h.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height, round, step})
}

func (h *TDMMsgHandler) handleTimeout(ti timeoutInfo, rs types.RoundState) {
	if ti.Height != h.Height || ti.Round < h.Round || (ti.Round == h.Round && ti.Step < h.Step) {
		return
	}

	h.mtx.Lock()
	defer h.mtx.Unlock()

	switch ti.Step {
	case types.RoundStepNewHeight:
		h.enterNewRound(ti.Height, 0)
	case types.RoundStepNewRound:
		h.timeState.EndConsumeTime(h)
		h.enterPropose(ti.Height, ti.Round)
	case types.RoundStepPropose:
		h.timeState.EndConsumeTime(h)
		h.enterPrevote(ti.Height, ti.Round)
	case types.RoundStepPrevote:
		h.timeState.EndConsumeTime(h)
		h.enterPrevoteWait(ti.Height, ti.Round)
	case types.RoundStepPrevoteWait:
		h.enterPrecommit(ti.Height, ti.Round)
	case types.RoundStepPrecommit:
		h.timeState.EndConsumeTime(h)
		h.enterPrecommitWait(ti.Height, ti.Round)
	case types.RoundStepPrecommitWait:
		h.enterNewRound(ti.Height, ti.Round+1)
	default:
		ConsLog.Warningf(LOGTABLE_CONS, "Invalid timeout step: %v", ti.Step)
	}
}

//recv from other peer info
func (h *TDMMsgHandler) processOuterMsg(tdmMsg *cmn.TDMMessage, pubKeyID []byte, peerID uint64) error {

	var err error
	switch tdmMsg.Type {
	case cmn.TDMType_MsgProposal:
		err = h.HandleOuterProposalMsg(tdmMsg, pubKeyID)
	case cmn.TDMType_MsgVote:
		err = h.HandleOutterVoteMsg(tdmMsg, pubKeyID)
	case cmn.TDMType_MsgBlockPart:
		err = h.HandleBlockPartMsg(tdmMsg)
	case cmn.TDMType_MsgProposalHeartBeat:
		err = h.HandleProposalHeartBeatMsg(tdmMsg)
	case cmn.TDMType_MsgNewRoundStep:
		err = h.HandleNewRoundStepMsg(tdmMsg)
	case cmn.TDMType_MsgCommitStep:
		err = h.HandleCommitStepMsg(tdmMsg)
	case cmn.TDMType_MsgProposalPOL:
		err = h.HandleProposalPOLMsg(tdmMsg)
	case cmn.TDMType_MsgHasVote:
		err = h.HandleHasVoteMsg(tdmMsg)
	case cmn.TDMType_MsgVoteSetMaj23:
		err = h.HandleVoteSetMaj23Msg(tdmMsg)
	case cmn.TDMType_MsgVoteSetBits:
		err = h.HandleVoteSetBitsMsg(tdmMsg)
	case cmn.TDMType_MsgHeightReq:
		err = h.SendBlockHeightReqToPeer(tdmMsg, peerID)
	case cmn.TDMType_MsgHeihtResp:
		err = h.ReciveHeihtResp(tdmMsg, peerID)
	default:
		ConsLog.Warningf(LOGTABLE_CONS, "(processOuterMsg) type %v not supported", tdmMsg.Type)
	}

	if err != nil {
		return fmt.Errorf("(processOuterMsg) handle tdmMsg err %v", err)
	}

	return nil
}

func (h *TDMMsgHandler) processInnerMsg(tdmMsg *cmn.TDMMessage, pubkeyID []byte) error {

	var err error
	switch tdmMsg.Type {
	case cmn.TDMType_MsgProposal:
		err = h.HandleInnerProposalMsg(tdmMsg)
	case cmn.TDMType_MsgVote:
		vote, _ := h.buildTypeFromVoteMsg(tdmMsg)
		ConsLog.Infof(LOGTABLE_CONS, "(processInnerMsg) VoteMsg height=%v,round=%v,vote=%v", vote.Height, vote.Round, vote)
		err = h.HandleInnerVoteMsg(tdmMsg, pubkeyID)

	case cmn.TDMType_MsgBlockPart:
		err = h.HandleBlockPartMsg(tdmMsg)

	case cmn.TDMType_MsgProposalHeartBeat:
		err = h.HandleProposalHeartBeatMsg(tdmMsg)

	case cmn.TDMType_MsgNewRoundStep:
		err = h.HandleNewRoundStepMsg(tdmMsg)

	case cmn.TDMType_MsgCommitStep:
		err = h.HandleCommitStepMsg(tdmMsg)

	case cmn.TDMType_MsgProposalPOL:
		err = h.HandleProposalPOLMsg(tdmMsg)

	case cmn.TDMType_MsgHasVote:
		err = h.HandleHasVoteMsg(tdmMsg)

	case cmn.TDMType_MsgVoteSetMaj23:
		err = h.HandleVoteSetMaj23Msg(tdmMsg)

	case cmn.TDMType_MsgVoteSetBits:
		err = h.HandleVoteSetBitsMsg(tdmMsg)

	default:
		ConsLog.Warningf(LOGTABLE_CONS, "(processOuterMsg) type %v not supported", tdmMsg.Type)
	}

	if err != nil {
		return fmt.Errorf("(processOuterMsg) handle tdmMsg err %v", err)
	}

	return nil
}

func (h *TDMMsgHandler) getBftGroupID() uint64 {
	return h.Validators.BgID
}

func (h *TDMMsgHandler) recvForkSearch() {
	h.allRoutineExitWg.Add(1)
	var err error
	for {
		select {
		case forkSearchMsg := <-h.ForkSearchMsgQueue:

			var consensusMsg = &consensusType.ConsensusMsg{}
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err = json.Unmarshal(forkSearchMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(dbftMgr) unmarshal conMsg err %v", err)
				continue
			}

			tdmMsg := &cmn.TDMMessage{}
			if err := json.Unmarshal(consensusMsg.Payload, tdmMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(msgDeliver) unmarshal tdmMsg err %v", err)
				continue
			}
			switch tdmMsg.Type {
			case cmn.TDMType_MsgBinaryBlockHashReq:
				err = h.forkSearchWorker.SendBinaryBlockHashResp(tdmMsg, forkSearchMsg.PeerID)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "forkSearch sendBinaryBlockHashResp err %v", err)
				}

			case cmn.TDMType_MsgBinaryBlockHashResp:
				select {
				case h.forkSearchWorker.ResChan <- tdmMsg:
				default:
					ConsLog.Warningf(LOGTABLE_CONS, "forkSearch ResChan full")
				}
			}
		case <-h.Quit():
			ConsLog.Infof(LOGTABLE_CONS, "forkSearch rountine stopped")
			h.allRoutineExitWg.Done()
			return
		}
	}
}
