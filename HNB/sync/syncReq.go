package sync

import (
	"fmt"
	syncComm "github.com/HNB-ECO/HNB-Blockchain/HNB/sync/common"
)

func (sh *SyncHandler) GetSyncState(chainID string) (uint8, error) {
	sch := sh.getSyncHandlerByChainID(chainID)
	if sch == nil {
		return 0, fmt.Errorf("chainID(%s == nil)", chainID)
	}
	return sch.getSyncState(), nil
}

func (sch *syncChainHandler) getSyncState() uint8 {
	sch.RLock()
	r := sch.syncInfo.SyncState
	sch.RUnlock()
	return r
}

func (sch *syncChainHandler) setSyncState(syncFlag uint8, beginCursor uint64, endCursor uint64, peerId uint64) {
	sch.Lock()
	sch.syncInfo.SyncState = syncFlag
	sch.syncInfo.BeginCursor = beginCursor
	sch.syncInfo.EndCursor = endCursor
	sch.syncInfo.ChainID = sch.chainID
	sch.syncInfo.FromID = peerId
	sch.Unlock()
}

func (sh *SyncHandler) SyncExit(chainID string) error {
	sch := sh.getSyncHandlerByChainID(chainID)
	if sch == nil {
		return fmt.Errorf("chainID(%s == nil)", chainID)
	}

	if sch.getSyncState() == 0 {
		return nil
	}

	syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(recv sync exit signal)", chainID)

	select {
	case sch.exitTask <- struct{}{}:
	default:
	}
	return nil
}

func (sh *SyncHandler) SyncToTarget(chainID string, beginCursor uint64,
	endCursor uint64, peerId uint64, isBlocked bool, version uint32, ntyCunc syncComm.NotifyFunc) error {

	syncLogger.Debugf(LOGTABLE_SYNC, "sync req chainID:%s, beginCursor:%d, endCursor:%d, isBlocked:%v",
		chainID, beginCursor, endCursor, isBlocked)

	sch := sh.getSyncHandlerByChainID(chainID)

	if sch == nil {
		sch = sh.addSyncHandler(chainID)
	}

	if sch.getSyncState() == 1 {
		return fmt.Errorf("chainID(%s).(syncing)", chainID)
	}

	sch.setSyncState(1, beginCursor, endCursor, peerId)

	var replyBlock chan error

	var retErr error
	if isBlocked == true {
		replyBlock = make(chan error)
	}

	req := &blockSyncReq{
		chainID:       chainID,
		beginIndex:    beginCursor,
		endIndex:      endCursor,
		replyChain:    replyBlock,
		peerID:        peerId,
		notifyHandler: ntyCunc,
		version:       version,
	}

	sh.syncReq <- req

	if isBlocked == true {
		select {
		case retErr = <-replyBlock:
		}
	}

	return retErr
}
