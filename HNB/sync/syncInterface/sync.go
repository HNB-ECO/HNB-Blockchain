package syncInterface

import (
	syncComm "HNB/sync/common"
)

type SyncServiceInf interface {
	SyncToTarget(chainID string, beginCursor uint64, endCursor uint64, peerId uint64, isBlocked bool, version uint32, ntyCunc syncComm.NotifyFunc) error

	GetSyncState(chainID string) (uint8, error)

	GetSyncInfo(chainID string) (*syncComm.SyncInfo, error)

	SyncExit(chainID string) error
}
