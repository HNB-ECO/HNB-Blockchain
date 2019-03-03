package types

import (
	"fmt"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
)

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote *Vote
}

// CommitStepMessage is sent when a block is committed.
type CommitStepMessage struct {
	Height           uint64
	BlockPartsHeader PartSetHeader
	BlockParts       *cmn.BitArray
}

// String returns a string representation.
func (m *CommitStepMessage) String() string {
	return fmt.Sprintf("[CommitStep H:%v BP:%v BA:%v]", m.Height, m.BlockPartsHeader, m.BlockParts)
}
