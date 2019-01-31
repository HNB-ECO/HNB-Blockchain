package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/types"
	"encoding/json"
)

func (h *TDMMsgHandler) HandleProposalPOLMsg(tdmMsg *cmn.TDMMessage) error {
	proposalPOLMsg := &cmn.ProposalPOLMessage{}
	err := json.Unmarshal(tdmMsg.Payload, proposalPOLMsg)
	if err != nil {
		return err
	}

	return nil
}

func (h *TDMMsgHandler) BuildProposalPOLMsg(rs types.RoundState) (*cmn.PeerMessage, error) {

	bitArray := &cmn.BitArray{
		Bits:  int(rs.Votes.Prevotes(rs.Proposal.POLRound).BitArray().Bits),
		Elems: rs.Votes.Prevotes(rs.Proposal.POLRound).BitArray().Elems,
	}

	proposalPOLMsg := &cmn.ProposalPOLMessage{
		Height:           rs.Height,
		ProposalPOLRound: rs.Proposal.POLRound,
		ProposalPOL:      bitArray,
	}

	proposalPOLData, err := json.Marshal(proposalPOLMsg)
	if err != nil {
		return nil, err
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: proposalPOLData,
		Type:    cmn.TDMType_MsgProposalPOL,
	}
	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		return nil, err
	}

	//tdmData, err := proto.Marshal(tdmMsg)
	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		return nil, err
	}

	peerMsg := &cmn.PeerMessage{
		Sender:      h.ID,
		Msg:         tdmData,
		IsBroadCast: false,
	}

	return peerMsg, nil
}
