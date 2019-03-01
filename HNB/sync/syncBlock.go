package sync

import (
	syncComm "github.com/HNB-ECO/HNB-Blockchain/HNB/sync/common"
)

type blockSyncReq struct {
	chainID       string
	beginIndex    uint64
	endIndex      uint64
	replyChain    chan error
	peerID        uint64
	notifyHandler syncComm.NotifyFunc
	version       uint32
}

func (sh *SyncHandler) blockSyncThread() error {
	syncLogger.Infof(LOGTABLE_SYNC, "* sync thread start")

	for {
		select {
		case req := <-sh.syncReq:

			if req == nil {
				continue
			}

			sch := sh.getSyncHandlerByChainID(req.chainID)

			if sch == nil {
				break
			}

			sch.blockSyncProcess(req)
		}

	}
}

func (sch *syncChainHandler) clearTaskChannel() {
	for {
		select {
		case <-sch.exitTask:
		default:
			return
		}
	}
}

func (sch *syncChainHandler) blockSyncProcess(req *blockSyncReq) {

	sch.clearTaskChannel()

	syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(pro sync req (%d,%d) %v)",
		req.chainID,
		req.beginIndex,
		req.endIndex,
		req.peerID)

	sch.notifyHandler = req.notifyHandler

	errSync := sch.syncBlock(req)

	// TODO
	if errSync != nil {
		notify := &syncComm.SyncNotify{FinishFlag: false, Version: req.version, Block: nil, SenderID: req.peerID}
		sch.notifyHandler(notify)
	} else {

		notify := &syncComm.SyncNotify{FinishFlag: true, Version: req.version, Block: nil, SenderID: req.peerID}
		sch.notifyHandler(notify)
	}

	if req.replyChain != nil {
		req.replyChain <- errSync
	}

	sch.setSyncState(0, 0, 0, 0)

	sch.notifyHandler = nil
}

func (sch *syncChainHandler) syncBlock(req *blockSyncReq) error {

	var syncOver bool = false
	endIndex := req.endIndex
	for {

		interval, err := sch.GetInterval(req.peerID)
		if err != nil || (req.endIndex-req.beginIndex) > 20 {
			interval = int(sch.sh.getMaxSyncBlockCount())
			sch.SetInterval(req.peerID, interval)
		}

		if (endIndex - req.beginIndex + 1) > uint64(interval) {
			req.endIndex = req.beginIndex + uint64(interval) - 1
		} else {
			req.endIndex = endIndex
		}

		blocks, errSync := sch.getRemoteBlocks(req.beginIndex, req.endIndex, req.peerID)

		if errSync != nil {
			syncLogger.Infof(LOGTABLE_SYNC, "chainID(%s) sync blks err %v", sch.chainID, errSync)
			return errSync
		}

		if blocks == nil && errSync == nil {
			syncLogger.Infof(LOGTABLE_SYNC, "chainID(%s) sync blks==nil", sch.chainID)
			return nil
		}

		//err = sch.sh.lg.CheckBlocks(sch.chainID, blocks)

		//if err == nil {

		length := len(blocks)
		req.endIndex = blocks[length-1].Header.BlockNum

		syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(sub sync blks(%d,%d)).(succ)",
			sch.chainID,
			req.beginIndex,
			req.endIndex)

		req.beginIndex = req.endIndex + 1

		if req.beginIndex > endIndex {
			syncOver = true
		}

		if blocks != nil {
			if sch.notifyHandler != nil {
				syncLogger.Debugf(LOGTABLE_SYNC, "chainID(%s) notify sync blk(%d->%d)", sch.chainID, blocks[0].Header.BlockNum, blocks[len(blocks)-1].Header.BlockNum)
				for _, v := range blocks {
					notify := &syncComm.SyncNotify{FinishFlag: false,
						Version: req.version, SenderID: req.peerID}
					notify.Block = v
					sch.notifyHandler(notify)

					syncLogger.Debugf(LOGTABLE_SYNC, "chainID(%s) notify sync blk(%d)", sch.chainID, v.Header.BlockNum)
				}
			} else {
				syncLogger.Errorf(LOGTABLE_SYNC, "chainID(%s).(notify handler = nil)", sch.chainID)
				break
			}

		}

		if syncOver {
			syncLogger.Infof(LOGTABLE_SYNC, "chainID(%s) sync end -> blk(%d)", sch.chainID, endIndex)
			break
		}

		//} else {
		//	syncLogger.Warningf(LOGTABLE_SYNC,"chainID(%s).(chk hash).(%s)", sch.chainID, err)
		//	continue
		//}

	}

	return nil
}
