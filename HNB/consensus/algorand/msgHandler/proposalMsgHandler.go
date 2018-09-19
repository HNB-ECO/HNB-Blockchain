package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/types"
	"time"
	"encoding/json"
)

/**
 * @FileName: proposalMsgHandler.go
 */
func (h *TDMMsgHandler) HandleInnerProposalMsg(tdmMsg *cmn.TDMMessage) error {

	proposalMsg := &cmn.ProposalMessage{}
	err := json.Unmarshal(tdmMsg.Payload, proposalMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "#(%v-%v) (HandleOuterProposalMsg) Error,err=%v", h.Height, h.Round, err)
		return err
	}
	ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) (HandleInnerProposalMsg) proposalMsg=(%v/%v),detail=%v", h.Height, h.Round, proposalMsg.Height, proposalMsg.Round, proposalMsg)
	proposal := h.buildTypeProposalFromNetMsg(proposalMsg)
	h.setProposal(proposal)

	return nil
}

// add for verify proposal message
func (h *TDMMsgHandler) VerifyProposal(tdmMsg *cmn.TDMMessage, peerId uint64) (*cmn.ProposalMessage, error) {
	proposalMsg := &cmn.ProposalMessage{}
	err := json.Unmarshal(tdmMsg.Payload, proposalMsg)
	if err != nil {
		return nil, err
	}
	sign := proposalMsg.Signature
	proposalMsg.Signature = nil
	//TODO verify proposal

	//
	//pk, err := h.Network.GetPK(peerId.Name)
	//if err != nil {
	//	return nil, err
	//}
	//data, err := proto.Marshal(proposalMsg)
	//if err != nil {
	//	return nil, err
	//}
	//ok, err := h.coor.Verify(pk, data, sign)
	//if err != nil {
	//	return nil, err
	//}
	//if !ok {
	//	return nil, fmt.Errorf("verify error ")
	//}
	proposalMsg.Signature = sign
	return proposalMsg, nil
}

func (h *TDMMsgHandler) HandleOuterProposalMsg(tdmMsg *cmn.TDMMessage, pubKeyID []byte) error {
	proposalMsg := &cmn.ProposalMessage{}
	err := json.Unmarshal(tdmMsg.Payload, proposalMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "#(%v-%v) (HandleOuterProposalMsg) Error,err=%v", h.Height, h.Round, err)
		return err
	}

	proposal := h.buildTypeProposalFromNetMsg(proposalMsg)
	h.setProposal(proposal)
	return nil
}

func (h *TDMMsgHandler) buildTypeProposalFromNetMsg(proposalMsg *cmn.ProposalMessage) *types.Proposal {
	proposal := &types.Proposal{
		Height:    proposalMsg.Height,
		Round:     proposalMsg.Round,
		Timestamp: time.Unix(int64(proposalMsg.Timestamp/1000), int64(proposalMsg.Timestamp%1000)),
		POLRound:  proposalMsg.PoLRound,
		POLBlockID: types.BlockID{
			Hash: proposalMsg.PoLBlockID.Hash,
			PartsHeader: types.PartSetHeader{
				Hash:  proposalMsg.PoLBlockID.PartsHeader.Hash,
				Total: int(proposalMsg.PoLBlockID.PartsHeader.Total),
			},
		},
		BlockPartsHeader: types.PartSetHeader{
			Total: int(proposalMsg.BlockPartsHeader.Total),
			Hash:  proposalMsg.BlockPartsHeader.Hash,
		},
	}

	return proposal
}

func (h *TDMMsgHandler) setProposal(proposal *types.Proposal) error {
	// Already have one
	// possibly catch double proposals
	if h.Proposal != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "#(%v-%v) (setProposal) not nil  proposal=(%v/%v) return nil", h.Height, h.Round, proposal.Height, proposal.Round)
		return nil
	}

	// Does not apply
	if proposal.Height != h.Height || proposal.Round != h.Round {
		ConsLog.Errorf(LOGTABLE_CONS, "#(%v-%v) (setProposal) not equal  proposal=(%v/%v)", h.Height, h.Round, proposal.Height, proposal.Round)
		return nil
	}

	// We don't care about the proposal if we're already in cstypes.RoundStepCommit.
	if types.RoundStepCommit <= h.Step {
		ConsLog.Errorf(LOGTABLE_CONS, "#(%v-%v) (setProposal) Step=%v err  proposal=(%v/%v)", h.Height, h.Round, h.Step, proposal.Height, proposal.Round)
		return nil
	}

	h.Proposal = proposal
	h.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockPartsHeader)
	return nil
}
