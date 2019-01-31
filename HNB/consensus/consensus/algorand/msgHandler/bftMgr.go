package msgHandler

import (
	appComm "HNB/appMgr/common"
	"HNB/config"
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/state"
	"HNB/consensus/algorand/types"
	"HNB/consensus/consensusManager/comm/consensusType"
	"HNB/ledger"
	"HNB/msp"
	"HNB/p2pNetwork"
	"HNB/p2pNetwork/message/reqMsg"
	"HNB/txpool"
	"HNB/util"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/json-iterator/go"
	"sync"
	"sync/atomic"
	"time"
)

var messageQueueSize = 19999
var BgDemandReceiveChanSize = 1999

const VP = "1"

type BftMgr struct {
	cmn.BaseService

	EventMsgQueue    chan *cmn.PeerMessage
	PeerMsgQueue     chan *cmn.PeerMessage
	InternalMsgQueue chan *cmn.PeerMessage
	TotalValidators *types.ValidatorSet
	BftNumber uint8
	ID uint64
	CandidateID uint64
	CurBftGroup BftGroup
	BgCandidate      map[uint64][]*cmn.BftGroupSwitchAdvice
	bgCandidateMutex sync.RWMutex
	BgAdvice      map[uint64][]*cmn.BftGroupSwitchAdvice
	bgAdviceMutex sync.RWMutex
	BgDemand      map[uint64]*cmn.BftGroupSwitchDemand
	bgDemandMutex sync.RWMutex
	bgChangeCache map[uint64]bool
	BlkTimeout int64
	BlkTimeoutTimer *time.Timer
	BlkTimeoutCount uint8
	CurHeight uint64
	MsgHandler *TDMMsgHandler
	BgDemandTimeout int64
	BgDemandReceiveChan  map[uint64]chan uint64
	bgDemandReceiveMutex sync.RWMutex
}

func NewBftMgr(lastCommitState state.State) (*BftMgr, error) {

	curBftGroup := BftGroup{
		BgID:       lastCommitState.Validators.BgID,
		Validators: lastCommitState.Validators.Validators,
		VRFValue:   lastCommitState.PrevVRFValue,
		VRFProof:   lastCommitState.PrevVRFProof,
	}

	bftMgr := &BftMgr{
		BgCandidate:         make(map[uint64][]*cmn.BftGroupSwitchAdvice, 31),
		BgAdvice:            make(map[uint64][]*cmn.BftGroupSwitchAdvice, 31),
		BgDemand:            make(map[uint64]*cmn.BftGroupSwitchDemand, 31),
		bgChangeCache:       make(map[uint64]bool, 1),
		BgDemandReceiveChan: make(map[uint64]chan uint64, 1),
		EventMsgQueue:       make(chan *cmn.PeerMessage, messageQueueSize),
		PeerMsgQueue:        make(chan *cmn.PeerMessage, messageQueueSize),
		InternalMsgQueue:    make(chan *cmn.PeerMessage, messageQueueSize),
		CurBftGroup:         curBftGroup,
		BgDemandTimeout:     int64(config.Config.BgDemandTimeout),
		BlkTimeout:          int64(config.Config.BlkTimeout),
		BftNumber:           uint8(config.Config.BftNum),
	}

	bftMgr.CurHeight = lastCommitState.LastBlockNum + 1
	bftMgr.ID = curBftGroup.BgID
	bftMgr.CandidateID = curBftGroup.BgID + 1

	totalVals, err := bftMgr.LoadTotalValidators()
	if err != nil {
		return nil, err
	}
	if len(totalVals.Validators) == 0 {
		return nil, errors.New("total vp empty")
	}

	bftMgr.TotalValidators = totalVals

	ConsLog.Infof(LOGTABLE_CONS,
		"(bgInfo) totalVals %v, bftVals %v, curHeight %d bgNum %d bgCandidateNum %d",
		bftMgr.TotalValidators, curBftGroup.Validators,
		bftMgr.CurHeight, bftMgr.ID, bftMgr.CandidateID)

	p2pNetwork.RegisterConsNotify(bftMgr.RecvConsMsg)

	bftMgr.BaseService = *cmn.NewBaseService("bftMgr", bftMgr)

	return bftMgr, nil
}

func (bftMgr *BftMgr) LoadTotalValidators() (*types.ValidatorSet, error) {
	totalVals := make([]*types.Validator, 0)
	for _, v := range config.Config.GeneValidators {
		address := msp.AccountPubkeyToAddress1(msp.StringToBccspKey(v.PubKeyStr))
		val := &types.Validator{
			PubKeyStr:   v.PubKeyStr,
			Address:     types.Address(address.GetBytes()),
			VotingPower: 1,
		}
		totalVals = append(totalVals, val)
	}

	return types.NewValidatorSet(totalVals, bftMgr.ID, nil, nil, nil), nil
}

func (bftMgr *BftMgr) RecvConsMsg(msg []byte, msgSender uint64) error {
	if msg == nil {
		ConsLog.Errorf(LOGTABLE_CONS, "cons recv msg = nil")
		return errors.New("cons recv msg = nil")
	}

	pm := cmn.PeerMessage{}
	err := json.Unmarshal(msg, &pm)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "unmarshal %v", err.Error())
		return err
	}

	pm.PeerID = msgSender

	select {
	case bftMgr.PeerMsgQueue <- &pm:
	default:
		ConsLog.Warningf(LOGTABLE_CONS, "cons recv msg chan full")
	}
	return nil
}

func (bftMgr *BftMgr) BroadcastMsgToAllVP(msg *cmn.PeerMessage) {
	ConsLog.Infof(LOGTABLE_CONS, "broad msg:%v", *msg)
	select {
	case bftMgr.EventMsgQueue <- msg:
	default:
		go func() { bftMgr.EventMsgQueue <- msg }()
	}
}

func (bftMgr *BftMgr) OnStart() error {
	go bftMgr.BroadcastMsgRoutineListener()
	go bftMgr.PeerMsgRoutineListener()
	go bftMgr.BlkTimeoutRoutineScanner()
	go bftMgr.CheckBgChange()
	err := bftMgr.MsgHandler.Start()
	if err != nil {
		return err
	}
	return nil
}

func (bftMgr *BftMgr) OnStop() {
	bftMgr.MsgHandler.OnStop()
}

func (bftMgr *BftMgr) CheckBgChange() {
	time.Sleep(time.Second * 3)
	var blkNum uint64 = 0
	var lastHeightChange uint64 = 0
	status, err := bftMgr.MsgHandler.LoadLastCommitStateFromCurHeight()
	if err != nil {
		panic("checkBgChange" + err.Error())
	} else {
		blkNum = status.LastBlockNum + 1
		lastHeightChange = status.LastHeightValidatorsChanged
	}

	for {
		blk, err := ledger.GetBlock(blkNum)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		if blk == nil {
			time.Sleep(3 * time.Second)
			continue
		}

		status, err := bftMgr.MsgHandler.LoadLastCommitStateFromBlk(blk)
		if err != nil {
			ConsLog.Infof(LOGTABLE_CONS, "%d get commit state err %s", blkNum, err)
			time.Sleep(3 * time.Second)
			continue
		}

		changeHeight := status.LastHeightValidatorsChanged
		if changeHeight > lastHeightChange {
			bg := BftGroup{
				BgID:       status.Validators.BgID,
				Validators: status.Validators.Validators,
				VRFProof:   status.PrevVRFProof,
				VRFValue:   status.PrevVRFValue,
			}

			_, ok := bftMgr.bgChangeCache[changeHeight]
			if !ok {
				ConsLog.Infof(LOGTABLE_CONS, "got val change .")
				bftMgr.ResetBftGroup(bg)
			} else {
				delete(bftMgr.bgChangeCache, changeHeight)
			}

			lastHeightChange = changeHeight
		}

		blkNum++
	}
}

func (bftMgr *BftMgr) BroadcastMsgRoutineListener() {
	ConsLog.Infof(LOGTABLE_CONS, "broadcast msg routine start")
	for {
		select {
		case broadcastMsg := <-bftMgr.EventMsgQueue:
			msg := reqMsg.NewConsMsg(broadcastMsg.Msg)
			p2pNetwork.Xmit(msg, true)
		}
	}
}

func (bftMgr *BftMgr) PeerMsgRoutineListener() {
	ConsLog.Infof(LOGTABLE_CONS, "receive peer msg routine start")
	for {
		select {
		case peerMsg := <-bftMgr.PeerMsgQueue:
			if peerMsg == nil {
				ConsLog.Infof(LOGTABLE_CONS, "recv msg is nil")
				continue
			}

			var consensusMsg = &consensusType.ConsensusMsg{}
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err := json.Unmarshal(peerMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(bftMgr) unmarshal conMsg err %v", err)
				continue
			}

			tdmMsg := &cmn.TDMMessage{}
			if err := json.Unmarshal(consensusMsg.Payload, tdmMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "unmarshal tdmMsg err %v", err)
				continue
			}

			ConsLog.Debugf(LOGTABLE_CONS, "recv peerMsg<-%s type %s", msp.PeerIDToString(peerMsg.Sender), tdmMsg.Type)

			err = bftMgr.MsgHandler.Verify(tdmMsg, peerMsg.Sender)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(bftMgr) Verify tdmMsg err %v", err)
				continue
			}

			if cmn.TDMType_MsgBgAdvice == tdmMsg.Type {
				err := bftMgr.HandleBgAdviceMsg(tdmMsg, peerMsg.PeerID)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "handle bgAdvice err %v", err)
					continue
				}

			} else if cmn.TDMType_MsgBgDemand == tdmMsg.Type {
				bgDemand, err := bftMgr.HandleBgDemand(tdmMsg)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "invalid bgDemand err %v", err)
					continue
				}
				bftMgr.PutBgDemand(bgDemand)
				if receiveChan, ok := bftMgr.GetBgDemandReceiveChan(bgDemand.BftGroupNum); ok {
					receiveChan <- bgDemand.BftGroupNum
				}

			} else if cmn.TDMType_MsgHeightReq == tdmMsg.Type || cmn.TDMType_MsgHeihtResp == tdmMsg.Type {
				bftMgr.MsgHandler.SyncMsgQueue <- peerMsg
			} else if bftMgr.MsgHandler.IsRunning() {
				bftMgr.MsgHandler.PeerMsgQueue <- peerMsg
			}
		case bgMsg := <-bftMgr.InternalMsgQueue:

			var consensusMsg = &consensusType.ConsensusMsg{}
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err := json.Unmarshal(bgMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(bftMgr) unmarshal conMsg err %v", err)
				continue
			}

			tdmMsg := &cmn.TDMMessage{}
			if err := json.Unmarshal(consensusMsg.Payload, tdmMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(bftMgr) internal unmarshal tdmMsg err %v", err)
				continue
			}

			if cmn.TDMType_MsgBgAdvice == tdmMsg.Type {
				err := bftMgr.HandleBgAdviceMsg(tdmMsg, bgMsg.PeerID)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "(bgAdvice) internal handle bgAdvice err %v", err)
				}

			} else if cmn.TDMType_MsgBgDemand == tdmMsg.Type {
				bgDemand, err := bftMgr.HandleBgDemand(tdmMsg)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) invalid bgDemand err %v", err)
					continue
				}
				bftMgr.PutBgDemand(bgDemand)
				if receiveChan, ok := bftMgr.GetBgDemandReceiveChan(bgDemand.BftGroupNum); ok {
					receiveChan <- bgDemand.BftGroupNum
				}

			}
		}
	}
}

func (bftMgr *BftMgr) BlkTimeoutRoutineScanner() {
	ConsLog.Infof(LOGTABLE_CONS, "(bftMgr) blk timeout scanner routine start")

	bftMgr.BlkTimeoutTimer = time.NewTimer(time.Duration(bftMgr.BlkTimeout) * time.Second)
	for {
		select {
		case <-bftMgr.BlkTimeoutTimer.C:
			h, err := ledger.GetBlockHeight()
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(blkTimeout) get blk height err %v", err)
				bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)
				continue
			}

			ConsLog.Infof(LOGTABLE_CONS, "(blkTimeout) read height %d last height %d", h, bftMgr.CurHeight)
			if h != bftMgr.CurHeight {
				bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)
				bftMgr.CurHeight = h
			} else if txpool.IsTxsLenZero(appComm.HNB) == false {
				if bftMgr.TotalValidators.Size() <= len(bftMgr.CurBftGroup.Validators) {
					bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)
					continue
				}
				// 防止交易刚进来，刚好卡上超时点，等待下一个超时点
				if bftMgr.BlkTimeoutCount == 1 {
					ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) blkTimeout && has txs req changBg curBgNum %d candidateBgNum %d", bftMgr.ID, bftMgr.CandidateID)

					bftMgr.MsgHandler.Stop()
					ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) wait msgHandler all routines stop")
					bftMgr.MsgHandler.allRoutineExitWg.Wait()
					ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) msgHandler all routines stop success")
					err := bftMgr.ReqBgChange(h)
					if err != nil {
						ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) reqBgChange err %v", err)
						atomic.AddUint64(&bftMgr.CandidateID, 1)
						err = bftMgr.MsgHandler.Reset()
						if err != nil {
							ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) msgHandler reset err %v curBgNum %d candidateBgNum %d", err, bftMgr.ID, bftMgr.CandidateID)
						}

						err = bftMgr.MsgHandler.Start()
						if err != nil {
							ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) start msgHandler err %v, curBgNum %d", err, bftMgr.ID)
						}

						ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) noNewBg coming start msgHandler try sync or cons")
					}
				} else {
					ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) blkTimeout && has txs req changBg curBgNum %d candidateBgNum %d,but we will wait for next", bftMgr.ID, bftMgr.CandidateID)
					bftMgr.BlkTimeoutCount = 1
				}

				bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)

			} else {
				ConsLog.Debugf(LOGTABLE_CONS, "(blkTimeout) reset timer")

				bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)
			}

		}
	}
}

func (bftMgr *BftMgr) ReqBgChange(height uint64) error {
	bgNum := bftMgr.CandidateID
	bgAdviceMsg, err := bftMgr.BuildBgAdvice(height, bgNum)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) build bgAdvice err %v bgNum %d", err, bgNum)
		return err
	}

	bftMgr.BroadcastMsgToAllVP(bgAdviceMsg)
	ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) broadcast bgAdvice bgNum %d", bgNum)

	bgAdviceMsg.PeerID = p2pNetwork.GetLocatePeerID()

	select {
	case bftMgr.InternalMsgQueue <- bgAdviceMsg:
		ConsLog.Infof(LOGTABLE_CONS, "(bgAdvice) myself bgNum %d", bgNum)
	default:
		ConsLog.Warningf(LOGTABLE_CONS, "tdm recvMsgChan full")
	}

	bftMgr.PutBgDemandReceiveChan(bgNum)
	err = bftMgr.WaitBgDemand(bgNum)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) waitBgDemand err %v bgNum %d", err, bgNum)
		return err
	}

	ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) BgDemand success bgNum %d", bgNum)
	return nil
}

func (bftMgr *BftMgr) ResetBftGroup(newBg BftGroup) error {

	err := bftMgr.MsgHandler.Stop()
	if err != nil {
		ConsLog.Warningf(LOGTABLE_CONS, "(resetBg) msgHandler already stop err %v", err)
	} else {
		ConsLog.Infof(LOGTABLE_CONS, "(resetBg) msgHandler is running stop first, curBgNum %d newBgNum %d", bftMgr.ID, newBg.BgID)
	}

	ConsLog.Infof(LOGTABLE_CONS, "(resetBg) wait msgHandler all routines stop")
	bftMgr.MsgHandler.allRoutineExitWg.Wait()
	ConsLog.Infof(LOGTABLE_CONS, "(resetBg) msgHandler all routines stop success")

	ConsLog.Infof(LOGTABLE_CONS, "(resetBg) reset msgHandler state, curBgNum %d newBgNum %d", bftMgr.ID, newBg.BgID)
	err = bftMgr.MsgHandler.Reset()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(resetBg) reset msgHandler state err %v, curBgNum %d newBgNum %d", err, bftMgr.ID, newBg.BgID)
		return err
	}

	bftMgr.CurBftGroup = newBg

	bftMgr.MsgHandler.Validators = types.NewValidatorSet(newBg.Validators, newBg.BgID, newBg.VRFValue, newBg.VRFProof, nil)
	bftMgr.MsgHandler.Votes = types.NewBlkNumVoteSet(bftMgr.MsgHandler.Height, bftMgr.MsgHandler.Validators)
	bftMgr.MsgHandler.LastCommitState.Validators = bftMgr.MsgHandler.Validators
	bftMgr.ID = newBg.BgID

	totalVals, err := bftMgr.LoadTotalValidators()
	if err != nil {
		return err
	}
	bftMgr.TotalValidators = totalVals

	if bftMgr.CurBftGroup.Exist(bftMgr.MsgHandler.digestAddr) {
		bftMgr.MsgHandler.LastCommitState.LastHeightValidatorsChanged = bftMgr.MsgHandler.Height
	}

	err = bftMgr.MsgHandler.Start()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(resetBg) start msgHandler err %v, curBgNum %d newBgNum %d", bftMgr.ID, newBg.BgID)
		return err
	}
	ConsLog.Infof(LOGTABLE_CONS, "(resetBg) start msgHandler success, curBgNum %d newBgNum %d, "+
		"curHeight %d, curVals:%v, totalVals:%v", bftMgr.ID, newBg.BgID,
		bftMgr.MsgHandler.Height, bftMgr.MsgHandler.Validators, totalVals)

	bftMgr.BlkTimeoutCount = 0
	bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)
	ConsLog.Infof(LOGTABLE_CONS, "(resetBg) reset blkTimeout")

	atomic.StoreUint64(&bftMgr.CandidateID, newBg.BgID+1)
	bftMgr.bgChangeCache[bftMgr.MsgHandler.Height] = true

	prevBgNum := newBg.BgID - 1
	bftMgr.CleanBgAdvice(prevBgNum)
	bftMgr.CleanBgDemand(prevBgNum)
	bftMgr.CleanBgCandidate(prevBgNum)

	return nil
}

func (bftMgr *BftMgr) OnReset() error {
	return nil
}

func (bftMgr *BftMgr) MakeNewBgFromBgAdvice(bgDemand *cmn.BftGroupSwitchDemand) (*BftGroup, error) {
	bgNum := bgDemand.BftGroupNum
	bgAdviceSlice := bgDemand.NewBftGroup
	newValidatorSlice := make([]*types.Validator, bftMgr.BftNumber)
	for index, advice := range bgAdviceSlice {
		validator := bftMgr.GetValidatorByAddr(util.HexBytes(advice.DigestAddr))
		if validator == nil {
			return nil, fmt.Errorf("(newBgEle) bgNum %d addr %X not exist", bgNum, advice.DigestAddr)
		}
		newValidatorSlice[index] = validator
	}
	ConsLog.Infof(LOGTABLE_CONS, "make newBg bgNum %d newVals %v len %d", bgNum, newValidatorSlice, len(newValidatorSlice))

	return &BftGroup{
		BgID:       bgNum,
		Validators: newValidatorSlice,
		VRFProof:   bgDemand.VRFProof,
		VRFValue:   bgDemand.VRFValue,
	}, nil
}

func (bftMgr *BftMgr) GetValidatorByAddr(addr util.HexBytes) *types.Validator {
	for _, val := range bftMgr.TotalValidators.Validators {
		if bytes.Equal(val.Address, addr) {
			return val.Copy()
		}
	}

	return nil
}

func (bftMgr *BftMgr) WaitBgDemand(bgNum uint64) error {
	ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) wait bg demand")

	receiveChan, _ := bftMgr.GetBgDemandReceiveChan(bgNum)

	bgDemandTimer := time.NewTimer(time.Duration(bftMgr.BgDemandTimeout) * time.Second)
	for {
		select {
		case <-bgDemandTimer.C:
			bftMgr.DelBgDemandReceiveChan(bgNum)
			return fmt.Errorf("(newBgEle) wait bg demand timeout")

		case receBgNum := <-receiveChan:
			if receBgNum == bftMgr.CandidateID-1 {
				ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) got bgDemand bgNum %d", receBgNum)
				bftMgr.DelBgDemandReceiveChan(receBgNum)
				return nil
			}
		}
	}
}

func (bftMgr *BftMgr) HasBgAdvice(addr util.HexBytes, bgNum uint64) bool {
	bgAdviceSlice := bftMgr.GetBgAdviceSlice(bgNum)
	if bgAdviceSlice == nil {
		return false
	}

	for _, advice := range bgAdviceSlice {
		if advice.BftGroupNum == bgNum &&
			bytes.Equal(addr, advice.DigestAddr) {
			return true
		}
	}

	return false
}

func (bftMgr *BftMgr) HasBgCandidate(addr util.HexBytes, bgNum uint64) bool {
	bgCandidateSlice := bftMgr.GetBgCandidate(bgNum)
	if bgCandidateSlice == nil {
		return false
	}

	for _, candidate := range bgCandidateSlice {
		if candidate.BftGroupNum == bgNum &&
			bytes.Equal(addr, candidate.DigestAddr) {
			return true
		}
	}

	return false
}

func (bftMgr *BftMgr) PutBgAdvice(bgAdvice *cmn.BftGroupSwitchAdvice) {
	if bftMgr.HasBgAdvice(bgAdvice.DigestAddr, bgAdvice.BftGroupNum) {
		ConsLog.Warningf(LOGTABLE_CONS, "(bftMgr) in adviceList receive same bgAdvice %v", bgAdvice)
		return
	}

	bftMgr.bgAdviceMutex.Lock()
	defer bftMgr.bgAdviceMutex.Unlock()

	_, ok := bftMgr.BgAdvice[bgAdvice.BftGroupNum]
	if !ok {
		bftMgr.BgAdvice[bgAdvice.BftGroupNum] = make([]*cmn.BftGroupSwitchAdvice, 0)
	}

	bftMgr.BgAdvice[bgAdvice.BftGroupNum] = append(bftMgr.BgAdvice[bgAdvice.BftGroupNum], bgAdvice)
}

func (bftMgr *BftMgr) GetBgAdviceSlice(bgGroupNum uint64) []*cmn.BftGroupSwitchAdvice {
	bftMgr.bgAdviceMutex.RLock()
	defer bftMgr.bgAdviceMutex.RUnlock()

	return bftMgr.BgAdvice[bgGroupNum]
}

func (bftMgr *BftMgr) PutBgDemandReceiveChan(bgNum uint64) {
	bftMgr.bgDemandReceiveMutex.Lock()
	defer bftMgr.bgDemandReceiveMutex.Unlock()

	bftMgr.BgDemandReceiveChan[bgNum] = make(chan uint64)
}

func (bftMgr *BftMgr) GetBgDemandReceiveChan(bgNum uint64) (chan uint64, bool) {
	bftMgr.bgDemandReceiveMutex.RLock()
	defer bftMgr.bgDemandReceiveMutex.RUnlock()

	receChan, ok := bftMgr.BgDemandReceiveChan[bgNum]
	return receChan, ok
}

func (bftMgr *BftMgr) DelBgDemandReceiveChan(bgNum uint64) {
	bftMgr.bgDemandReceiveMutex.Lock()
	defer bftMgr.bgDemandReceiveMutex.Unlock()

	delete(bftMgr.BgDemandReceiveChan, bgNum)
}

func (bftMgr *BftMgr) CleanBgAdvice(bgNum uint64) {
	bftMgr.bgAdviceMutex.Lock()
	defer bftMgr.bgAdviceMutex.Unlock()

	delete(bftMgr.BgAdvice, bgNum)
}

func (bftMgr *BftMgr) PutBgDemand(bgDemand *cmn.BftGroupSwitchDemand) {
	bftMgr.bgDemandMutex.Lock()
	defer bftMgr.bgDemandMutex.Unlock()

	bftMgr.BgDemand[bgDemand.BftGroupNum] = bgDemand
}

func (bftMgr *BftMgr) GetBgDemand(bgGroupNum uint64) *cmn.BftGroupSwitchDemand {
	bftMgr.bgDemandMutex.RLock()
	defer bftMgr.bgDemandMutex.RUnlock()

	return bftMgr.BgDemand[bgGroupNum]
}

func (bftMgr *BftMgr) CleanBgDemand(bgNum uint64) {
	bftMgr.bgDemandMutex.Lock()
	defer bftMgr.bgDemandMutex.Unlock()

	delete(bftMgr.BgDemand, bgNum)
}

func (bftMgr *BftMgr) PutBgCandidate(bgAdvice *cmn.BftGroupSwitchAdvice) {
	if bftMgr.HasBgCandidate(bgAdvice.DigestAddr, bgAdvice.BftGroupNum) {
		ConsLog.Warningf(LOGTABLE_CONS, "(bftMgr) in candidateList receive same bgAdvice %v", bgAdvice)
		return
	}

	bftMgr.bgCandidateMutex.Lock()
	defer bftMgr.bgCandidateMutex.Unlock()

	_, ok := bftMgr.BgCandidate[bgAdvice.BftGroupNum]
	if !ok {
		bftMgr.BgCandidate[bgAdvice.BftGroupNum] = make([]*cmn.BftGroupSwitchAdvice, 0)
	}

	bftMgr.BgCandidate[bgAdvice.BftGroupNum] = append(bftMgr.BgCandidate[bgAdvice.BftGroupNum], bgAdvice)
}

func (bftMgr *BftMgr) GetBgCandidate(bgGroupNum uint64) []*cmn.BftGroupSwitchAdvice {
	bftMgr.bgCandidateMutex.RLock()
	defer bftMgr.bgCandidateMutex.RUnlock()

	return bftMgr.BgCandidate[bgGroupNum]
}

func (bftMgr *BftMgr) CleanBgCandidate(bgNum uint64) {
	bftMgr.bgCandidateMutex.Lock()
	defer bftMgr.bgCandidateMutex.Unlock()

	delete(bftMgr.BgCandidate, bgNum)
}
