package common

import (
	ledger "HNB/ledger/blockStore/common"
)

type SyncInfo struct {
	ChainID     string `json:"chainID"`
	SyncState   uint8  `json:"syncState"`
	BeginCursor uint64 `json:"beginCursor"`
	EndCursor   uint64 `json:"endCursor"`
	FromID      uint64 `json:"fromID"`
}

type SyncNotify struct {
	SenderID   uint64
	Block      *ledger.Block
	FinishFlag bool
	Version    uint32
}

type NotifyFunc func(*SyncNotify)

type SyncEventType int32

const (
	SyncEventType_REQUEST_BLOCKS_EVENT  SyncEventType = 0
	SyncEventType_RESPONSE_BLOCKS_EVENT SyncEventType = 1
)

type RequestBlocks struct {
	//    SyncEventType type = 1;
	ChainID    string `json:"chainId,omitempty"`
	BeginIndex uint64 `json:"startIndex,omitempty"`
	EndIndex   uint64 `json:"endIndex,omitempty"`
	Version    uint64 `json:"version,omitempty"`
}

type ResponseBlocks struct {
	ChainID string          `json:"chainId,omitempty"`
	Blocks  []*ledger.Block `json:"blocks,omitempty"`
	Version uint64          `json:"version,omitempty"`
}

type SyncMessage struct {
	Type    SyncEventType `json:"type,omitempty"`
	Payload []byte        `json:"payload,omitempty"`
	Sender  uint64        `json:"sender,omitempty"`
}

func (m *ResponseBlocks) GetBlocks() []*ledger.Block {
	if m != nil {
		return m.Blocks
	}
	return nil
}
