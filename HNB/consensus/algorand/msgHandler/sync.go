package msgHandler

import (
	tdmComm "HNB/consensus/algorand/common"
	"HNB/consensus/consensusManager/comm/consensusType"
	"HNB/ledger"
	"HNB/sync"
	psync "HNB/sync/common"
	"fmt"
	"github.com/json-iterator/go"
)

type SyncHandler struct {
	BeginBlockNum uint64
	EndBlockNum   uint64
	SenderId      uint64
	Version       uint32
}
type SyncBlkCount struct {
	SyncBlkCountTarget   *tdmComm.AtomicIndex
	SyncBlkCountComplete *tdmComm.AtomicIndex
}

func (h *TDMMsgHandler) RecvSync(msg *psync.SyncNotify) {
	ConsLog.Infof(CONSENSUS, "RecvSync msg for %s", msg.SenderID)
	h.recvSyncChan <- msg
}

func (h *TDMMsgHandler) SyncToTarget() error {
	err := sync.SyncToTarget(CHAINID, h.syncHandler.BeginBlockNum, h.syncHandler.EndBlockNum,
		h.syncHandler.SenderId, false, h.syncHandler.Version, h.RecvSync)

	if err != nil {
		ConsLog.Warningf(CONSENSUS, "sync error %s", err)
		return err
	}
	return nil
}

func (h *TDMMsgHandler) syncServer() {
	h.allRoutineExitWg.Add(1)
	for {
		select {
		case syncMsg := <-h.SyncMsgQueue:

			var consensusMsg = &consensusType.ConsensusMsg{}
			var json = jsoniter.ConfigCompatibleWithStandardLibrary
			err := json.Unmarshal(syncMsg.Msg, &consensusMsg)
			if err != nil {
				ConsLog.Errorf(CONSENSUS, "(dbftMgr) unmarshal conMsg err %v", err)
				continue
			}

			tdmMsg := &tdmComm.TDMMessage{}
			if err := json.Unmarshal(consensusMsg.Payload, tdmMsg); err != nil {
				ConsLog.Errorf(CONSENSUS, "(msgDeliver) unmarshal tdmMsg err %v", err)
				continue
			}

			err = h.processOuterMsg(tdmMsg, syncMsg.Sender, syncMsg.PeerID)
			if err != nil {
				ConsLog.Errorf(CONSENSUS, "(msgDeliver) %s process sync peerMsg<-%s err %v", CHAINID, syncMsg.Sender, err)
			}
		case <-h.syncTimer.C:
			ConsLog.Warningf(CONSENSUS, "%s sync timeout v %d", CHAINID, h.syncHandler.Version)
			h.isSyncStatus.SetFalse()
			err := sync.SyncExit(CHAINID)
			if err != nil {
				ConsLog.Errorf(CONSENSUS, "sync exit err %s", err)
			}
			h.syncHandler.Version++

		case <-h.Quit():
			ConsLog.Infof(CONSENSUS, "(msgHandler) sync receive stopped")
			h.allRoutineExitWg.Done()
			return
		}
	}
}

func (h *TDMMsgHandler) resetSync() {
	h.syncHandler.Version++
	h.isSyncStatus.SetFalse()
	h.stopSyncTimer()
}

func (h *TDMMsgHandler) SyncEntrance(toHeight uint64, sendID uint64) {
	if h.isSyncStatus.Get() {
		ConsLog.Infof(CONSENSUS, "peer in syncing, return")
	} else {
		h.syncHandler.Version++

		if toHeight > 10 && (toHeight-10) >= h.Height {
			h.updateSyncBlkCount(10)
			h.updateSync(h.Height, h.Height+9, sendID)
		} else {
			h.updateSyncBlkCount(toHeight - h.Height)
			h.updateSync(h.Height, toHeight-1, sendID)
		}

		h.startSyncTimer()
	}
}

func (h *TDMMsgHandler) CheckBlock() {

}

func (h *TDMMsgHandler) stopSyncTimer() {
	ConsLog.Infof(CONSENSUS, "stop sync timer...")
	h.syncTimer.Stop()
}

func (h *TDMMsgHandler) startSyncTimer() {
	ConsLog.Infof(CONSENSUS, "start sync timer...")
	h.syncTimer.Reset(h.syncTimeout)
}
func (h *TDMMsgHandler) updateSyncBlkCount(syncNum uint64) {
	syncBlkCnt := &SyncBlkCount{
		SyncBlkCountTarget:   tdmComm.NewAtomicIndex(syncNum),
		SyncBlkCountComplete: tdmComm.NewAtomicIndex(0),
	}
	h.cacheSyncBlkCount[CHAINID] = syncBlkCnt
	ConsLog.Infof(CONSENSUS, "set-sync-mark[%d, %d]", h.cacheSyncBlkCount[CHAINID].SyncBlkCountTarget, h.cacheSyncBlkCount[CHAINID].SyncBlkCountComplete)
}

func (h *TDMMsgHandler) updateSync(beginBlockNum, endBlockNum uint64, reciver uint64) {
	var err error = nil
	defer func() {
		if err != nil {
			h.isSyncStatus.SetFalse()
			h.stopSyncTimer()
			ConsLog.Infof(CONSENSUS, "update sync error %s,status set %t", err, h.isSyncStatus.Get())
		}
	}()
	ConsLog.Infof(CONSENSUS, "tdm state is isSyncStatus %t", h.isSyncStatus.Get())
	if h.isSyncStatus.Get() {
		return
	} else {
		h.isSyncStatus.SetTrue()

		pID := reciver
		ConsLog.Infof(CONSENSUS, "(sync blk) [%d,%d] from %s v %d", beginBlockNum, endBlockNum, pID, h.syncHandler.Version)
		h.syncHandler.BeginBlockNum = beginBlockNum
		h.syncHandler.EndBlockNum = endBlockNum
		h.syncHandler.SenderId = pID

		err = h.SyncToTarget()
	}
}

func (h *TDMMsgHandler) BeginToSync(blkNum uint64, reciver uint64) error {
	if reciver < 0 {
		return fmt.Errorf("target peer is nilf")
	}
	localNum, err := ledger.GetBlockHeight()
	if err != nil {
		return err
	}
	if localNum >= blkNum {
		return fmt.Errorf("target %d local %d not need to sync", blkNum, localNum)
	}
	h.updateSync(localNum+1, blkNum, reciver)
	return nil
}
