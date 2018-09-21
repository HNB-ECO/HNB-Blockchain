package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/types"
	"HNB/consensus/algorand/state"
	"HNB/p2pNetwork/message/reqMsg"
	"HNB/config"
	"errors"
	"bytes"
	"sync"
	"time"
	"fmt"
	"HNB/p2pNetwork"
	"encoding/json"
	"HNB/ledger"
	"HNB/txpool"
	"HNB/msp"
	"sync/atomic"
)

var messageQueueSize = 19999
var BgDemandReceiveChanSize = 1999

const VP = "1"





type BftMgr struct {
	cmn.BaseService

	//广播到其他节点的消息
	EventMsgQueue    chan *cmn.PeerMessage
	PeerMsgQueue     chan *cmn.PeerMessage
	InternalMsgQueue chan *cmn.PeerMessage
	// VP 节点总列表
	TotalValidators *types.ValidatorSet
	// bft 共识节点数
	BftNumber uint8
	// 当前共识组编号
	ID uint64
	// 候选共识组编号
	CandidateID uint64
	// 当前共识组
	CurBftGroup BftGroup
	// 候选共识组map k:共识组编号 v:候选节点地址索引
	BgCandidate      map[uint64][]*cmn.BftGroupSwitchAdvice
	bgCandidateMutex sync.RWMutex
	// 请求更换共识组消息map
	BgAdvice      map[uint64][]*cmn.BftGroupSwitchAdvice
	bgAdviceMutex sync.RWMutex
	// 历史共识组列表
	BgDemand      map[uint64]*cmn.BftGroupSwitchDemand
	bgDemandMutex sync.RWMutex
	// 在共识组节点切换的时候，是否重新加载过
	bgChangeCache map[uint64]bool
	// 接收块超时时间, 单位秒
	BlkTimeout int64
	// 接收块超时 超时器
	BlkTimeoutTimer *time.Timer
	// 超时计数器，只能未0或1
	// 每次有新共识组时，变为0, 过了一次变成1
	BlkTimeoutCount uint8
	// 当前高度
	CurHeight uint64
	// 消息处理
	MsgHandler *TDMMsgHandler
	// 接收请求更换共识组列表超时时间, 单位秒
	BgDemandTimeout int64
	// 接收bgDemand消息管道，供己方发起的换共识组使用，每发起一次，新加一个管道
	BgDemandReceiveChan  map[uint64]chan uint64
	bgDemandReceiveMutex sync.RWMutex
}




func NewBftMgr(lastCommitState state.State) (*BftMgr, error) {

	//设定当前共识组
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

	//当前高度为块号 + 1
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

	//ConsLog = logging.GetLogIns()

	bftMgr.TotalValidators = totalVals

	ConsLog.Infof(LOGTABLE_CONS,
		"(bgInfo) totalVals %v, bftVals %v, curHeight %d bgNum %d bgCandidateNum %d",
			bftMgr.TotalValidators, curBftGroup.Validators,
				bftMgr.CurHeight, bftMgr.ID, bftMgr.CandidateID)


	p2pNetwork.RegisterConsNotify(bftMgr.RecvConsMsg)

	bftMgr.BaseService = *cmn.NewBaseService("bftMgr", bftMgr)


	return bftMgr, nil
}

//获取参与共识的总节点数, 需要从配置文件读取
func (bftMgr *BftMgr) LoadTotalValidators() (*types.ValidatorSet, error) {
	totalVals := make([]*types.Validator, 0)
	for _,v := range config.Config.GeneValidators{
		val := &types.Validator{
			PubKeyStr:  v.PubKeyStr,
			Address: types.Address(v.PubKeyStr),
			VotingPower: 1,
		}
		totalVals = append(totalVals, val)
	}

	return types.NewValidatorSet(totalVals, bftMgr.ID, nil, nil), nil
}

// 接收到network模块过来的共识消息
func (bftMgr *BftMgr) RecvConsMsg(msg []byte) {
	if msg == nil {
		ConsLog.Errorf(LOGTABLE_CONS, "cons recv msg = nil")
		return
	}

	pm := cmn.PeerMessage{}
	err := json.Unmarshal(msg, &pm)
	if err != nil{
		ConsLog.Errorf(LOGTABLE_CONS, "unmarshal %v", err.Error())
		return
	}

	select {
	case bftMgr.PeerMsgQueue <- &pm:
	default:
		ConsLog.Warningf(LOGTABLE_CONS, "cons recv msg chan full")
	}

}

// 广播更换共识组的消息, 这里向所有VP节点广播
func (bftMgr *BftMgr) BroadcastMsgToAllVP(msg *cmn.PeerMessage) {
	ConsLog.Infof(LOGTABLE_CONS, "broad msg:%v", *msg)
	select {
	case bftMgr.EventMsgQueue <- msg:
	default:
		go func() { bftMgr.EventMsgQueue <- msg }()
	}
}

// 在启动时，本地节点设置接收块超时
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

// 在停止时，停止共识消息处理
func (bftMgr *BftMgr) OnStop() {
	bftMgr.MsgHandler.OnStop()
}

// 检测是否共识组已经变更了，供落后的节点发现
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

// 广播消息监听协程，这里的消息需要向所有VP广播(目前时广播给所有节点 无论vp 还是 nvp)
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

// 监听接收消息处理协程, 接收更换共识组的消息，共识消息
func (bftMgr *BftMgr) PeerMsgRoutineListener() {
	ConsLog.Infof(LOGTABLE_CONS, "receive peer msg routine start")
	for {
		select {
		//接收到其他节点消息
		case peerMsg := <-bftMgr.PeerMsgQueue:
			if peerMsg == nil {
				ConsLog.Infof(LOGTABLE_CONS, "recv msg is nil")
				continue
			}
			//所有消息
			tdmMsg := &cmn.TDMMessage{}
			if err := json.Unmarshal(peerMsg.Msg, tdmMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "unmarshal tdmMsg err %v", err)
				continue
			}

			ConsLog.Debugf(LOGTABLE_CONS, "recv peerMsg<-%s type %s", msp.GetPubStrFromPeerID(peerMsg.Sender), tdmMsg.Type)

			//验证发送者身份
			err := bftMgr.MsgHandler.Verify(tdmMsg, peerMsg.Sender)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(bftMgr) Verify tdmMsg err %v", err)
				continue
			}

			if cmn.TDMType_MsgBgAdvice == tdmMsg.Type {
				// 接收请求更换共识组消息
				err := bftMgr.HandleBgAdviceMsg(tdmMsg, peerMsg.Sender)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "handle bgAdvice err %v", err)
					continue
				}

			} else if cmn.TDMType_MsgBgDemand == tdmMsg.Type {
				// 接收请求更换共识组列表消息
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
			tdmMsg := &cmn.TDMMessage{}
			if err := json.Unmarshal(bgMsg.Msg, tdmMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_CONS, "(bftMgr) internal unmarshal tdmMsg err %v", err)
				continue
			}

			//接收到自己的更换共识组消息
			if cmn.TDMType_MsgBgAdvice == tdmMsg.Type {
				// 接收请求更换共识组消息
				err := bftMgr.HandleBgAdviceMsg(tdmMsg, bgMsg.Sender)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_CONS, "(bgAdvice) internal handle bgAdvice err %v", err)
				}

			} else if cmn.TDMType_MsgBgDemand == tdmMsg.Type {
				// 接收请求更换共识组列表消息
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

// 块超时监听协程
func (bftMgr *BftMgr) BlkTimeoutRoutineScanner() {
	ConsLog.Infof(LOGTABLE_CONS, "(bftMgr) blk timeout scanner routine start")

	bftMgr.BlkTimeoutTimer = time.NewTimer(time.Duration(bftMgr.BlkTimeout) * time.Second)
	for {
		select {
		case <-bftMgr.BlkTimeoutTimer.C:
			// 判断当前高度与之前缓存高度，若大于，重新设置超时
			// 若等于，且该链有交易 广播发起请求更换共识组消息
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
			} else if txpool.TxsLen(txpool.HGS) != 0 {
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
				// 重置块超时
				ConsLog.Debugf(LOGTABLE_CONS, "(blkTimeout) reset timer")

				bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)
			}

		}
	}
}

// 请求更换共识组，包括发起bgAdvice和等待接收bgDemand
func (bftMgr *BftMgr) ReqBgChange(height uint64) error {
	bgNum := bftMgr.CandidateID
	bgAdviceMsg, err := bftMgr.BuildBgAdvice(height, bgNum)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) build bgAdvice err %v bgNum %d", err, bgNum)
		return err
	}

	// 广播
	bftMgr.BroadcastMsgToAllVP(bgAdviceMsg)
	ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) broadcast bgAdvice bgNum %d", bgNum)

	// 消息发给自己，统一处理
	select {
	case bftMgr.InternalMsgQueue <- bgAdviceMsg:
		ConsLog.Infof(LOGTABLE_CONS, "(bgAdvice) myself bgNum %d", bgNum)
	default:
		ConsLog.Warningf(LOGTABLE_CONS, "tdm recvMsgChan full")
	}

	bftMgr.PutBgDemandReceiveChan(bgNum)
	// 等待处理bgDemand
	err = bftMgr.WaitBgDemand(bgNum)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "(newBgEle) waitBgDemand err %v bgNum %d", err, bgNum)
		return err
	}

	ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) BgDemand success bgNum %d", bgNum)
	return nil
}

// 重新设置共识组
// 1. 在新共识组的，重新启动 msgHandler
// 2. 不在新共识组的，停止msgHandler
func (bftMgr *BftMgr) ResetBftGroup(newBg BftGroup) error {

	//if bftMgr.MsgHandler.IsRunning() { 不用判断 后续reset会重置
	err := bftMgr.MsgHandler.Stop()
	if err != nil {
		// 只会发生"已经停止"的错误,不用返回err
		ConsLog.Warningf(LOGTABLE_CONS,"(resetBg) msgHandler already stop err %v", err)
	} else {
		ConsLog.Infof(LOGTABLE_CONS,"(resetBg) msgHandler is running stop first, curBgNum %d newBgNum %d", bftMgr.ID, newBg.BgID)
	}

	// 等待所有协程停止完成
	ConsLog.Infof(LOGTABLE_CONS,"(resetBg) wait msgHandler all routines stop")
	bftMgr.MsgHandler.allRoutineExitWg.Wait()
	ConsLog.Infof(LOGTABLE_CONS,"(resetBg) msgHandler all routines stop success")

	// Reset 使得服务启动状态为0
	// Reset 之前必须先停止服务，服务停止状态为1
	// Reset 只重置了服务状态，并未启动服务, 重置 start =0 stop =0
	ConsLog.Infof(LOGTABLE_CONS,"(resetBg) reset msgHandler state, curBgNum %d newBgNum %d", bftMgr.ID, newBg.BgID)
	err = bftMgr.MsgHandler.Reset()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS,"(resetBg) reset msgHandler state err %v, curBgNum %d newBgNum %d", err, bftMgr.ID, newBg.BgID)
		return err
	}

	bftMgr.CurBftGroup = newBg
	// 设置新的参与共识的验证节点
	bftMgr.MsgHandler.Validators = types.NewValidatorSet(newBg.Validators, newBg.BgID, newBg.VRFValue, newBg.VRFProof)
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
	//	ConsLog.Infof(LOGTABLE_CONS,"(resetBg) need to participate consensus %v", bftMgr.MsgHandler.digestAddr)
	err = bftMgr.MsgHandler.Start()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS,"(resetBg) start msgHandler err %v, curBgNum %d newBgNum %d", bftMgr.ID, newBg.BgID)
		return err
	}
	ConsLog.Infof(LOGTABLE_CONS,"(resetBg) start msgHandler success, curBgNum %d newBgNum %d, "+
		"curHeight %d, curVals:%v, totalVals:%v", bftMgr.ID, newBg.BgID,
		bftMgr.MsgHandler.Height, bftMgr.MsgHandler.Validators, totalVals)
	//} else {
	//	ConsLog.Infof(LOGTABLE_CONS,"(resetBg) need not to participate consensus %v", bftMgr.MsgHandler.digestAddr)
	//}

	// 重置blkTimeouCount
	bftMgr.BlkTimeoutCount = 0
	bftMgr.BlkTimeoutTimer.Reset(time.Duration(bftMgr.BlkTimeout) * time.Second)
	ConsLog.Infof(LOGTABLE_CONS,"(resetBg) reset blkTimeout")

	//bftMgr.ID = newBg.BgID
	atomic.StoreUint64(&bftMgr.CandidateID, newBg.BgID+1)
	bftMgr.bgChangeCache[bftMgr.MsgHandler.Height] = true

	prevBgNum := newBg.BgID - 1
	bftMgr.CleanBgAdvice(prevBgNum)
	bftMgr.CleanBgDemand(prevBgNum)
	bftMgr.CleanBgCandidate(prevBgNum)

	return nil
}

// 暂未需要
func (bftMgr *BftMgr) OnReset() error {
	return nil
}

// 从bgDemand中获取新的共识组
func (bftMgr *BftMgr) MakeNewBgFromBgAdvice(bgDemand *cmn.BftGroupSwitchDemand) (*BftGroup, error) {
	bgNum := bgDemand.BftGroupNum
	bgAdviceSlice := bgDemand.NewBftGroup
	newValidatorSlice := make([]*types.Validator, bftMgr.BftNumber)
	for index, advice := range bgAdviceSlice {
		validator := bftMgr.GetValidatorByAddr(cmn.HexBytes(advice.DigestAddr))
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

func (bftMgr *BftMgr) GetValidatorByAddr(addr cmn.HexBytes) *types.Validator {
	for _, val := range bftMgr.TotalValidators.Validators {
		if bytes.Equal(val.Address, addr) {
			return val.Copy()
		}
	}

	return nil
}

// 等待接收请求更换共识组列表，直到超时
func (bftMgr *BftMgr) WaitBgDemand(bgNum uint64) error {
	ConsLog.Infof(LOGTABLE_CONS,"(newBgEle) wait bg demand")

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

// 判断是否已经发出请求更换共识组
func (bftMgr *BftMgr) HasBgAdvice(addr cmn.HexBytes, bgNum uint64) bool {
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

// 判断是否已经在候选者列表中
func (bftMgr *BftMgr) HasBgCandidate(addr cmn.HexBytes, bgNum uint64) bool {
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
		ConsLog.Warningf(LOGTABLE_CONS,"(bftMgr) in adviceList receive same bgAdvice %v", bgAdvice)
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


// 在本节点请求换共识组时放入
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

// 在本节点请求换共识组成功/超时后删除
func (bftMgr *BftMgr) DelBgDemandReceiveChan(bgNum uint64) {
	bftMgr.bgDemandReceiveMutex.Lock()
	defer bftMgr.bgDemandReceiveMutex.Unlock()

	delete(bftMgr.BgDemandReceiveChan, bgNum)
}

// 清除新共识组编号之前的缓存
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
		ConsLog.Warningf(LOGTABLE_CONS,"(bftMgr) in candidateList receive same bgAdvice %v", bgAdvice)
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
