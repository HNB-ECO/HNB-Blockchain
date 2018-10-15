package sync

import (
	syncComm "HNB/sync/common"
)

// server for other moudle

func SyncToTarget(chainID string, beginCursor uint64, endCursor uint64, peerId uint64, isBlocked bool, version uint32, ntyCunc syncComm.NotifyFunc) error {
	return sh.SyncToTarget(chainID, beginCursor, endCursor, peerId, isBlocked, version, ntyCunc)
}

func GetSyncState(chainID string) (uint8, error) {
	return sh.GetSyncState(chainID)
}

func GetSyncInfo(chainID string) (*syncComm.SyncInfo, error) {
	return sh.GetSyncInfo(chainID)
}

func SyncExit(chainID string) error {
	return sh.SyncExit(chainID)
}
