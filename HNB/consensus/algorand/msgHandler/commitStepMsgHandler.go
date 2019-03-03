package msgHandler

import (
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
)

func (h *TDMMsgHandler) HandleCommitStepMsg(tdmMsg *cmn.TDMMessage) error {

	return nil
}

func (ps *PeerState) ApplyCommitStepMessage(msg *types.CommitStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.ProposalBlockPartsHeader = msg.BlockPartsHeader
	ps.ProposalBlockParts = msg.BlockParts
}
