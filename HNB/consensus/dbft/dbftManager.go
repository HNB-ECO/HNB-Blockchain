package dbft

import (
	"encoding/json"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"github.com/json-iterator/go"

	tdmCmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/dbft/util"
	"time"

	"errors"
	appComm "github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr/common"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	dposCom "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/dbft/common"

	"bytes"

	tdmMsgHandler "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/msgHandler"

	tdmStatus "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/state"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/txpool"
	"sort"
)

const VP = "1"

var messageQueueSize = 19999

const (
	CONSENSUS = "CONSENSUS"
)

type DBFTManager struct {
	tdmCmn.BaseService
	ChainID                string
	EventMsgQueue          chan *cmn.PeerMessage
	PeerMsg4BroadcastQueue chan *cmn.PeerMessage
	PeerMsgQueue           chan *cmn.PeerMessage
	InternalMsgQueue       chan *cmn.PeerMessage
	TotalValidators        *types.ValidatorSet
	BlkTimeout             int64
	BlkTimeoutTimer        *time.Timer
	BlkTimeoutCount        uint8
	CurHeight              uint64

	BftNumber uint8

	PosVoteTable  string
	PosAssetTable string
	PosEpochTable string
	bftHandler    *tdmMsgHandler.TDMMsgHandler

	candidateNo        util.AtomicNo
	epoch              *Epoch
	epochList          *EpochListCache
	witnessesNum       int
	EpochChange        *EpochChangeMap
	NewEpoch           *NewEpochMap
	newEpochNotify     *NewEpochNotify
	epochChangeCache   *EpochChangeCache
	EpochChangeTimeout int64
	FullBlkChangeChan  chan uint64

	votedCache map[int64]bool

	BlkChangeChan chan *Epoch
}

func NewDBFTManager(lastCommitState tdmStatus.State) (*DBFTManager, error) {

	var epochInfo *Epoch

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal(lastCommitState.Validators.Extend, &epochInfo)
	if err != nil {
		return nil, fmt.Errorf("unmarshal Epoch err %s", err.Error())
	}
	epochInfo.WitnessList = lastCommitState.Validators.Validators

	dbftManager := &DBFTManager{
		EpochChange:        NewEpochChangeMap(),
		NewEpoch:           NewNewEpochMap(),
		epochChangeCache:   NewEpochChangeCache(),
		newEpochNotify:     NewNewEpochNotify(),
		EventMsgQueue:      make(chan *cmn.PeerMessage, messageQueueSize),
		PeerMsgQueue:       make(chan *cmn.PeerMessage, messageQueueSize),
		InternalMsgQueue:   make(chan *cmn.PeerMessage, messageQueueSize),
		ChainID:            appComm.HNB,
		PosAssetTable:      appComm.HNB + "_" + TABLE_POS_ASSET,
		PosEpochTable:      appComm.HNB + "_" + TABLE_EPOCH_INFO,
		PosVoteTable:       appComm.HNB + "_" + TABLE_POS_VOTE,
		epoch:              epochInfo,
		EpochChangeTimeout: int64(config.Config.BgDemandTimeout),
		BlkTimeout:         int64(config.Config.BlkTimeout),
		epochList:          NewEpochList(appComm.HNB),
		FullBlkChangeChan:  make(chan uint64),
		BlkChangeChan:      make(chan *Epoch),
		BftNumber:          uint8(config.Config.BftNum),
	}
	ConsLog.Infof(LOGTABLE_DBFT, "NewDBFTManager epoch %v", dbftManager.epoch)
	dbftManager.CurHeight = lastCommitState.LastBlockNum + 1
	dbftManager.candidateNo.SetNo(epochInfo.EpochNo + 1)
	dbftManager.epochList.SetEpoch(epochInfo)
	totalVals, err := dbftManager.LoadTotalValidators(0)
	if err != nil {
		return nil, err
	}
	if len(totalVals.Validators) == 0 {
		return nil, errors.New("total vp empty")
	}

	dbftManager.TotalValidators = totalVals

	ConsLog.Infof(LOGTABLE_DBFT, "(epochInfo) totalVals %v, bftVals %v, curHeight %d epochNo %d epochCandidateNum %d",
		dbftManager.TotalValidators, epochInfo.WitnessList, dbftManager.CurHeight,
		dbftManager.epoch.EpochNo, dbftManager.candidateNo.GetCurrentNo())

	p2pNetwork.RegisterConsNotify(dbftManager.RecvConsMsg)

	dbftManager.BaseService = *cmn.NewBaseService("dbftManager", dbftManager)
	return dbftManager, nil
}

func (dbftMgr *DBFTManager) OnStart() error {

	go dbftMgr.BroadcastMsgRoutineListener()
	go dbftMgr.PeerMsgRoutineListener()
	go dbftMgr.BlkTimeoutRoutineScanner()
	go dbftMgr.FullBlkChangeServer()
	go dbftMgr.FindEpochChange()

	err := dbftMgr.bftHandler.Start()
	if err != nil {
		return err
	}
	ConsLog.Infof(LOGTABLE_DBFT, "(dbftMgr) start msgHandler success, curBgNum %d", dbftMgr.epoch.EpochNo)

	return nil
}

func (dbftMgr *DBFTManager) OnStop() {
	dbftMgr.bftHandler.OnStop()
}

func (dbftMgr *DBFTManager) RecvConsMsg(msg []byte, msgSender uint64) error {
	if msg == nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "cons recv msg = nil")
		return errors.New("cons recv msg = nil")
	}

	pm := cmn.PeerMessage{}
	err := json.Unmarshal(msg, &pm)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "unmarshal %v", err.Error())
		return err
	}

	pm.PeerID = msgSender

	select {
	case dbftMgr.PeerMsgQueue <- &pm:
	default:
		ConsLog.Warningf(LOGTABLE_DBFT, "cons recv msg chan full")
	}
	return nil
}

func (dbftMgr *DBFTManager) BroadcastMsgRoutineListener() {
	ConsLog.Infof(LOGTABLE_DBFT, "broadcast msg routine start")
	for {
		select {
		case broadcastMsg := <-dbftMgr.EventMsgQueue:
			msg := reqMsg.NewConsMsg(broadcastMsg.Msg)
			p2pNetwork.Xmit(msg, true)
		}
	}
}

func (dbftMgr *DBFTManager) PeerMsgRoutineListener() {
	ConsLog.Infof(LOGTABLE_DBFT, "(dbftMgr) receive peer msg routine start")

	for {
		select {
		case peerMsg := <-dbftMgr.PeerMsgQueue:
			if peerMsg == nil {
				ConsLog.Warningf(LOGTABLE_DBFT, "(dbftMgr) peerMsg is nil")
				continue
			}
			var consensusMsg = &consensusType.ConsensusMsg{}
			err := json.Unmarshal(peerMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "(dbftMgr) unmarshal conMsg err %v", err)
				continue
			}
			if consensusMsg.Type == int(consensusType.DBFT) {
				dposMsg := &dposCom.DPoSMessage{}
				if err := json.Unmarshal(consensusMsg.Payload, dposMsg); err != nil {
					ConsLog.Errorf(LOGTABLE_DBFT, "(dbftMgr) unmarshal dposMsg err %v", err)
					continue
				}

				ConsLog.Infof(LOGTABLE_DBFT, "(dbftMgr) %s recv dpos peerMsg<-%s type %s", dbftMgr.ChainID, peerMsg.Sender, dposMsg.Type)

				err := dbftMgr.Verify(dposMsg, peerMsg.Sender)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_DBFT, "(dbftMgr) Verify dposMsg err %v", err)
					continue
				}
				ConsLog.Info(LOGTABLE_DBFT, "(dbftMgr) Verify dposMsg succ")

				if dbftMgr.ChainID != dposMsg.ChainId {
					ConsLog.Errorf("(dbftMgr) peerMsg ChainID reg(%s) != msg(%s)", dbftMgr.ChainID, dposMsg.ChainId)
				}

				if dposCom.DPoSType_MsgEpochChange == dposMsg.Type {
					err := dbftMgr.HandleEpochChange(dposMsg, peerMsg.PeerID)
					if err != nil {
						ConsLog.Errorf(LOGTABLE_DBFT, "(epochChange) handle epochChange err %v", err)
						continue
					}

				} else if dposCom.DPoSType_MsgNewEpoch == dposMsg.Type {
					newEpoch, err := dbftMgr.HandleNewEpoch(dposMsg)
					if err != nil {
						ConsLog.Errorf(LOGTABLE_DBFT, "(newBgEle) invalid newEpoch err %v", err)
						continue
					}

					dbftMgr.NewEpoch.SetNewEpochMap(newEpoch)
					if receiveChan, ok := dbftMgr.newEpochNotify.GetNewEpochNotify(newEpoch.EpochNo); ok {
						receiveChan <- newEpoch.EpochNo
					}

				}

			} else {
				tdmMsg := &tdmCmn.TDMMessage{}
				if err := json.Unmarshal(consensusMsg.Payload, tdmMsg); err != nil {
					ConsLog.Errorf(LOGTABLE_DBFT, "(bftMgr) unmarshal dposMsg err %v", err)
					continue
				}

				ConsLog.Debugf(LOGTABLE_DBFT, "(bftMgr) %s recv peerMsg<-%s type %d", dbftMgr.ChainID, msp.PeerIDToString(peerMsg.Sender), tdmMsg.Type)

				if tdmCmn.TDMType_MsgHeightReq == tdmMsg.Type || tdmCmn.TDMType_MsgHeihtResp == tdmMsg.Type {
					dbftMgr.bftHandler.SyncMsgQueue <- peerMsg
				} else if tdmCmn.TDMType_MsgBinaryBlockHashReq == tdmMsg.Type || tdmCmn.TDMType_MsgBinaryBlockHashResp == tdmMsg.Type {
					dbftMgr.bftHandler.ForkSearchMsgQueue <- peerMsg
				} else if dbftMgr.bftHandler.IsRunning() {
					dbftMgr.bftHandler.PeerMsgQueue <- peerMsg
				}
			}

		case epochMsg := <-dbftMgr.InternalMsgQueue:
			var consensusMsg = &consensusType.ConsensusMsg{}
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err := json.Unmarshal(epochMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "(dbftMgr) unmarshal conMsg err %v", err)
				continue
			}
			dposMsg := &dposCom.DPoSMessage{}
			if err := json.Unmarshal(consensusMsg.Payload, dposMsg); err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "(bftMgr) internal unmarshal dposMsg err %v", err)
				continue
			}

			if dposCom.DPoSType_MsgEpochChange == dposMsg.Type {
				err := dbftMgr.HandleEpochChange(dposMsg, epochMsg.PeerID)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_DBFT, "(epochChange) handle epochChange err %v", err)
					continue
				}

			} else if dposCom.DPoSType_MsgNewEpoch == dposMsg.Type {
				newEpoch, err := dbftMgr.HandleNewEpoch(dposMsg)
				if err != nil {
					ConsLog.Errorf(LOGTABLE_DBFT, "(newBgEle) invalid newEpoch err %v", err)
					continue
				}

				dbftMgr.NewEpoch.SetNewEpochMap(newEpoch)
				if receiveChan, ok := dbftMgr.newEpochNotify.GetNewEpochNotify(newEpoch.EpochNo); ok {
					receiveChan <- newEpoch.EpochNo
				}

			}
		}
	}
}

func (dbftMgr *DBFTManager) BlkTimeoutRoutineScanner() {
	ConsLog.Infof(LOGTABLE_DBFT, "(dbftMgr) blk timeout scanner routine start")

	dbftMgr.BlkTimeoutTimer = time.NewTimer(time.Duration(dbftMgr.BlkTimeout) * time.Second)
	for {
		select {
		case <-dbftMgr.BlkTimeoutTimer.C:
			h, err := ledger.GetBlockHeight()
			if err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "(blkTimeout) get blk height err %v", err)
				dbftMgr.BlkTimeoutTimer.Reset(time.Duration(dbftMgr.BlkTimeout) * time.Second)
				continue
			}

			ConsLog.Infof(LOGTABLE_DBFT, "(blkTimeout) read height %d last height %d", h, dbftMgr.CurHeight)
			if h != dbftMgr.CurHeight {
				ConsLog.Debugf(LOGTABLE_DBFT, "(blkTimeout) new height %d reset timer", h)
				dbftMgr.BlkTimeoutTimer.Reset(time.Duration(dbftMgr.BlkTimeout) * time.Second)
				dbftMgr.CurHeight = h

			} else if txpool.IsTxsLenZero(appComm.HNB) == false {
				if ok := dbftMgr.IsNeedCheckEpoch(); !ok {
					ConsLog.Debugf(LOGTABLE_DBFT, "no need to CheckEpoch, reset timer")
					dbftMgr.BlkTimeoutTimer.Reset(time.Duration(dbftMgr.BlkTimeout) * time.Second)
					continue
				}

				candidateBgNum := dbftMgr.candidateNo.GetCurrentNo()
				if dbftMgr.BlkTimeoutCount == 1 {
					ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) blkTimeout && has txs req changEpoch curEpochNum %d candidateEpochNum %d", dbftMgr.epoch.EpochNo, candidateBgNum)

					dbftMgr.bftHandler.Stop()
					ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) wait msgHandler all routines stop")
					dbftMgr.bftHandler.AllRoutineExitWg()
					ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) msgHandler all routines stop success")

					err := dbftMgr.ReqEpochChange(h)
					if err != nil {
						ConsLog.Errorf(LOGTABLE_DBFT, "(newEpochEle) reqEpochChange err %v", err)
						if dbftMgr.candidateNo.GetCurrentNo() == candidateBgNum {
							dbftMgr.candidateNo.GetNextNo()
						}

						err = dbftMgr.bftHandler.Reset()
						if err != nil {
							ConsLog.Errorf(LOGTABLE_DBFT, "(newEpochEle) msgHandler reset err %v curEpochNum %d candidateBgNum %d", err, dbftMgr.epoch.EpochNo, candidateBgNum)
						}

						err = dbftMgr.bftHandler.Start()
						if err != nil {
							ConsLog.Errorf(LOGTABLE_DBFT, "(newEpochEle) start msgHandler err %v, curEPochNum %d", err, dbftMgr.epoch.EpochNo)
						}

						ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) noNewBg coming start msgHandler try sync or cons")
					}

				} else {
					ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) blkTimeout && has txs req changEpoch curEPochNum %d candidateBgNum %d,but we will wait for next", dbftMgr.epoch.EpochNo, candidateBgNum)
					dbftMgr.BlkTimeoutCount = 1
				}

				dbftMgr.BlkTimeoutTimer.Reset(time.Duration(dbftMgr.BlkTimeout) * time.Second)

			} else {
				// 重置块超时
				ConsLog.Debugf(LOGTABLE_DBFT, "(blkTimeout) reset timer")

				dbftMgr.BlkTimeoutTimer.Reset(time.Duration(dbftMgr.BlkTimeout) * time.Second)
			}

		}
	}
}

func (dbftMgr *DBFTManager) FindEpochChange() {
	for {
		select {
		case newEpoch := <-dbftMgr.BlkChangeChan:
			ConsLog.Infof(LOGTABLE_DBFT, "FindEpochChange begin to ResetEpoch")
			err := dbftMgr.ResetEpoch(newEpoch)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "FindEpochChange err %s", err.Error())
			}
		}
	}
}

func (dbftMgr *DBFTManager) BroadcastMsgToAllVP(msg *cmn.PeerMessage) {
	ConsLog.Debugf(LOGTABLE_DBFT, "进入publishEvent:msg", msg)
	select {
	case dbftMgr.EventMsgQueue <- msg:
	default:
		ConsLog.Info(LOGTABLE_DBFT, "broadCast msg queue is full. Using a go-routine")
		go func() { dbftMgr.EventMsgQueue <- msg }()

	}
}

func (dbftMgr *DBFTManager) ReqEpochChange(height uint64) error {
	epochNum := dbftMgr.candidateNo.GetCurrentNo()
	epochChangeMsg, err := dbftMgr.BuildEpochChangeMsg(height, epochNum)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "(newEpochEle) build epochChange err %v epochNum %d", err, epochNum)
		return err
	}

	// 广播
	dbftMgr.BroadcastMsgToAllVP(epochChangeMsg)
	ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) broadcast epochChange epochNum %d", epochNum)

	// 消息发给自己，统一处理
	select {
	case dbftMgr.InternalMsgQueue <- epochChangeMsg:
		ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) myself epochNum %d", epochNum)
	default:
		ConsLog.Warningf(LOGTABLE_DBFT, "tdm recvMsgChan full")
	}

	dbftMgr.newEpochNotify.SetNewEpochNotify(epochNum)
	// 等待处理newEpoch
	err = dbftMgr.WaitNewEpoch(epochNum)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "(newEpochEle) waitnewEpoch err %v epochNum %d", err, epochNum)
		return err
	}

	ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) newEpoch success epochNum %d", epochNum)
	return nil
}

func (dbftMgr *DBFTManager) ResetEpoch(newEpoch *Epoch) error {
	ConsLog.Debugf(LOGTABLE_DBFT, "(resetEpoch) newEpochNo %d dependEpochNo %d", newEpoch.EpochNo, newEpoch.DependEpochNo)

	err := dbftMgr.unfreezeToken(dbftMgr.epoch.EpochNo)
	if err != nil {
		return err
	}

	err = dbftMgr.bftHandler.Stop()
	if err != nil {
		ConsLog.Warningf(LOGTABLE_DBFT, "(resetEpoch) msgHandler already stop err %v", err)
	} else {
		ConsLog.Infof(LOGTABLE_DBFT, "(resetEpoch) msgHandler is running stop first, curEpochNum %d newEpochNum %d", dbftMgr.epoch.EpochNo, newEpoch.EpochNo)
	}

	ConsLog.Infof(LOGTABLE_DBFT, "(resetEpoch) wait msgHandler all routines stop")
	dbftMgr.bftHandler.AllRoutineExitWg()
	ConsLog.Infof(LOGTABLE_DBFT, "(resetEpoch) msgHandler all routines stop success")

	ConsLog.Infof(LOGTABLE_DBFT, "(resetEpoch) reset msgHandler state, curEpochNum %d newEpochNum %d", dbftMgr.epoch.EpochNo, newEpoch.EpochNo)
	err = dbftMgr.bftHandler.Reset()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "(resetEpoch) reset msgHandler state err %v, curEpochNum %d newEpochNum %d", err, dbftMgr.epoch.EpochNo, newEpoch.EpochNo)
		return err
	}

	epochInfo := &Epoch{
		EpochNo:       newEpoch.EpochNo,
		DependEpochNo: newEpoch.DependEpochNo,
		BeginNum:      dbftMgr.bftHandler.Height,
		EndNum:        dbftMgr.bftHandler.Height,
		TargetBlkNum:  TARGET_BLK_COUNT,
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.Marshal(epochInfo)
	if err != nil {
		return err
	}
	dbftMgr.bftHandler.Validators = types.NewValidatorSet(newEpoch.WitnessList, newEpoch.EpochNo, nil, nil, data)
	dbftMgr.bftHandler.Votes = types.NewBlkNumVoteSet(dbftMgr.bftHandler.Height, dbftMgr.bftHandler.Validators)
	dbftMgr.bftHandler.LastCommitState.Validators = dbftMgr.bftHandler.Validators

	dbftMgr.bftHandler.LastCommitState.Validators.Extend = data
	dbftMgr.epoch = newEpoch

	ConsLog.Infof(LOGTABLE_DBFT, "LastCommitState  %v", dbftMgr.bftHandler.LastCommitState)
	totalVals, err := dbftMgr.LoadTotalValidators(newEpoch.EpochNo)
	if err != nil {
		return err
	}
	dbftMgr.TotalValidators = totalVals

	if dbftMgr.epoch.IsExist(dbftMgr.bftHandler.GetDigestAddr()) {
		dbftMgr.bftHandler.LastCommitState.LastHeightValidatorsChanged = dbftMgr.bftHandler.Height
	}

	dbftMgr.BlkTimeoutCount = 0
	dbftMgr.BlkTimeoutTimer.Reset(time.Duration(dbftMgr.BlkTimeout) * time.Second)
	ConsLog.Info(LOGTABLE_DBFT, "(resetEpoch) reset blkTimeout")

	dbftMgr.candidateNo.SetNo(newEpoch.EpochNo + 1)
	dbftMgr.epochChangeCache.SetCache(dbftMgr.bftHandler.Height)
	ConsLog.Infof(LOGTABLE_DBFT, "(resetEpoch) new epoch %v ", *newEpoch)
	dbftMgr.epochList.SetEpoch(newEpoch)

	prevBgNum := newEpoch.EpochNo - 1
	dbftMgr.EpochChange.DelEpochChange(prevBgNum)
	dbftMgr.NewEpoch.DelNewEpochMap(prevBgNum)
	dbftMgr.epochList.DelEpoch(prevBgNum)
	dbftMgr.ResetVoteCache()

	err = dbftMgr.bftHandler.Start()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "(resetEpoch) start msgHandler err %v, curEpochNum %d newEpochNum %d",
			dbftMgr.epoch.EpochNo, newEpoch.EpochNo)
		return err
	}
	ConsLog.Infof(LOGTABLE_DBFT, "(resetEpoch) start msgHandler success, curEpochNum %d newEpochNum %d, "+
		"curHeight %d, curVals:%v, totalVals:%v",
		dbftMgr.epoch.EpochNo, newEpoch.EpochNo,
		dbftMgr.bftHandler.Height, dbftMgr.bftHandler.Validators, totalVals)
	return nil
}

func (dbftMgr *DBFTManager) ResetVoteCache() {
	for k, _ := range dbftMgr.votedCache {
		delete(dbftMgr.votedCache, k)
	}
}

func (dbftMgr *DBFTManager) LoadTotalValidators(candidateEpochNo uint64) (*types.ValidatorSet, error) {
	totalVals := make([]*types.Validator, 0)
	if candidateEpochNo == 0 {
		for _, v := range config.Config.GeneValidators {
			address := msp.AccountPubkeyToAddress1(msp.StringToBccspKey(v.PubKeyStr))
			val := &types.Validator{
				PubKeyStr:   v.PubKeyStr,
				Address:     types.Address(address.GetBytes()),
				VotingPower: 1,
			}
			totalVals = append(totalVals, val)
		}
	} else {
		epochList := make([]string, 0)
		for {
			epochList = GetEpochInfo(candidateEpochNo)
			if len(epochList) <= 4 {
				ConsLog.Errorf(LOGTABLE_DBFT, "(GetEpochInfo from server len <4 for candidateEpochNo %d", candidateEpochNo)
				time.Sleep(1 * time.Second)
				continue
			}
			break
		}
		ConsLog.Infof(LOGTABLE_DBFT, "GetEpochInfo %v", epochList)
		for _, v := range epochList {
			address := msp.AccountPubkeyToAddress1(msp.StringToBccspKey(v))
			val := &types.Validator{
				PubKeyStr:   v,
				Address:     types.Address(address.GetBytes()),
				VotingPower: 1,
			}
			totalVals = append(totalVals, val)
		}

	}

	sort.Sort(types.ValidatorsByAddress(totalVals))
	return types.NewValidatorSet(totalVals, dbftMgr.epoch.EpochNo, nil, nil, nil), nil
}

func (dbftMgr *DBFTManager) WaitNewEpoch(epochNo uint64) error {
	ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) wait newEpoch")

	receiveChan, _ := dbftMgr.newEpochNotify.GetNewEpochNotify(epochNo)

	epochChangeTimer := time.NewTimer(time.Duration(dbftMgr.EpochChangeTimeout) * time.Second)
	for {
		select {
		case <-epochChangeTimer.C:
			dbftMgr.newEpochNotify.DelNewEpochNotify(epochNo)
			return fmt.Errorf("(newEpochEle) wait newEpoch timeout bgNum %d", epochNo)

		case receEpochNum := <-receiveChan:
			if receEpochNum == dbftMgr.candidateNo.GetCurrentNo()-1 {
				ConsLog.Infof(LOGTABLE_DBFT, "(newEpochEle) got newEpoch bgNum %d", receEpochNum)
				dbftMgr.newEpochNotify.DelNewEpochNotify(receEpochNum)
				return nil
			}
		}
	}
}

func (dbftMgr *DBFTManager) CheckEpochChange(tdmMsgHander *tdmMsgHandler.TDMMsgHandler) error {
	ConsLog.Infof(LOGTABLE_DBFT, "CheckEpochChange in")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	preData := dbftMgr.bftHandler.Validators.Extend
	var preEpochInfo *Epoch
	err := json.Unmarshal(preData, &preEpochInfo)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "CheckEpochChange Unmarshal preEpochInfo err %s", err.Error())
	}
	ConsLog.Infof(LOGTABLE_DBFT, "CheckEpochChange current %v preEpochInfo %v", dbftMgr.epoch, preEpochInfo)
	if dbftMgr.epoch.EpochNo != preEpochInfo.EpochNo {
		preEpochInfo.WitnessList = dbftMgr.bftHandler.Validators.Validators
		preEpochInfo.SetEndBlk(dbftMgr.bftHandler.Height)
		dbftMgr.bftHandler.IsContinueConsensus = false
		dbftMgr.BlkChangeChan <- preEpochInfo
		return nil
	}
	dbftMgr.epoch.EpochNo = preEpochInfo.EpochNo
	dbftMgr.epoch.DependEpochNo = preEpochInfo.DependEpochNo
	dbftMgr.epoch.WitnessList = dbftMgr.bftHandler.Validators.Validators
	dbftMgr.epoch.SetBeginBlk(preEpochInfo.BeginNum)
	dbftMgr.epoch.SetEndBlk(dbftMgr.bftHandler.Height)
	dbftMgr.epoch.TargetBlkNum = preEpochInfo.TargetBlkNum
	ConsLog.Infof(LOGTABLE_DBFT, "CheckEpochChange epoch %v", dbftMgr.epoch)
	if dbftMgr.epoch.IsBlksToTarget(TARGET_BLK_COUNT) {
		ConsLog.Infof(LOGTABLE_DBFT, "CheckEpochChange begin to change epoch")
		dbftMgr.bftHandler.IsContinueConsensus = false
		dbftMgr.FullBlkChangeChan <- dbftMgr.epoch.EpochNo
	} else {
		epochInfo := &Epoch{
			EpochNo:       dbftMgr.epoch.EpochNo,
			DependEpochNo: dbftMgr.epoch.DependEpochNo,
			BeginNum:      dbftMgr.epoch.BeginNum,
			EndNum:        dbftMgr.epoch.EndNum,
			TargetBlkNum:  TARGET_BLK_COUNT,
		}
		ConsLog.Infof(LOGTABLE_DBFT, "CheckEpochChange epochinfo %v", epochInfo)

		data, err := json.Marshal(epochInfo)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "Marshal err %s", err.Error())
			return err
		}
		dbftMgr.bftHandler.Validators.Extend = data
		if dbftMgr.epoch.GetEndBlk()-dbftMgr.epoch.GetBeginBlk() == 1 {
			epochInfo.WitnessList = dbftMgr.epoch.WitnessList
			ConsLog.Infof(LOGTABLE_DBFT, "CheckEpochChange RecordEpochInfo")
			err := dbftMgr.RecordEpochInfo(epochInfo)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "CheckEpochChange RecordEpochInfo err %s", err.Error())
			}
		}

		ConsLog.Infof(LOGTABLE_DBFT, "preData %v", data)
		dbftMgr.bftHandler.LastCommitState.Validators.Extend = data
		ConsLog.Infof(LOGTABLE_DBFT, "LastCommitState  v %v lv %v", dbftMgr.bftHandler.LastCommitState.Validators, dbftMgr.bftHandler.LastCommitState.LastValidators)
		if dbftMgr.bftHandler.LastCommitState.Validators != nil {
			ConsLog.Infof(LOGTABLE_DBFT, "dbftMgr.bftHandler.LastCommitState.Validators.Extend %v", dbftMgr.bftHandler.LastCommitState.Validators.Extend)
		}
	}
	ConsLog.Infof(LOGTABLE_DBFT, "CheckEpochChange out")
	return nil
}

func (dbftMgr *DBFTManager) FullBlkChangeServer() {
	for {
		select {
		case <-dbftMgr.FullBlkChangeChan:
			epoch, err := dbftMgr.BuildNewEpoch(dbftMgr.candidateNo.GetCurrentNo())
			if err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "(FullBlkChange) BuildNewEpoch err %s", err.Error())
				continue
			}
			epoch.EpochNo = dbftMgr.candidateNo.GetCurrentNo()
			epoch.ReSetBeginBlk(dbftMgr.bftHandler.Height)
			ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) begin to ResetEpoch")
			err = dbftMgr.ResetEpoch(epoch)
			if err != nil {
				ConsLog.Errorf(LOGTABLE_DBFT, "(resetEpoch) unfreeze token err %v", err)
			}
		}
	}
}

func (dbftMgr *DBFTManager) IsNeedCheckEpoch() bool {
	total, err := dbftMgr.LoadTotalValidators(dbftMgr.candidateNo.GetCurrentNo())
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "LoadTotalValidators err %s", err.Error())
		return false
	}

	if len(total.Validators) < len(dbftMgr.epoch.WitnessList) {
		return false
	}

	if len(total.Validators) > len(dbftMgr.epoch.WitnessList) {
		return true
	}

	var match bool
	for _, val := range total.Validators {
		match = false
		for _, selfWitness := range dbftMgr.epoch.WitnessList {
			if bytes.Equal(val.Hash(), selfWitness.Hash()) {
				match = true
				break
			}
		}
		if !match {
			return true
		}
	}
	return false
}
