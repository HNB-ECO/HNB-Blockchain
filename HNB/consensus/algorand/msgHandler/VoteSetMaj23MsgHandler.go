package msgHandler

import (
	"encoding/json"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
)

func (h *TDMMsgHandler) HandleVoteSetMaj23Msg(tdmMsg *cmn.TDMMessage) error {

	h.mtx.Lock()
	height, votes := h.Height, h.Votes
	h.mtx.Unlock()
	h.mtx.Lock()
	nrsMsg := &cmn.HasVoteMessage{}
	err := json.Unmarshal(tdmMsg.Payload, nrsMsg)
	if err != nil {
		return err
	}
	if height != nrsMsg.Height {
		return nil
	}
	// Peer claims to have a maj23 for some BlockID at H,R,S,
	errp := votes.SetPeerMaj23(nrsMsg.Round, byte(nrsMsg.VoteType), h.ID, h.LastCommitState.LastBlockID) //TODO 此blockId是否正确
	if errp != nil {
		return errp
	}
	// Respond with a VoteSetBitsMessage showing which votes we have.
	// (and consequently shows which we don't have)
	var ourVotes *cmn.BitArray
	switch nrsMsg.VoteType {
	case cmn.VoteType_Prevote:
		//ourVotes = votes.Prevotes(nrsMsg.Round).BitArrayByBlockID(nrsMsg.BlockID) //
		ourVotes = votes.Prevotes(nrsMsg.Round).BitArrayByBlockID(h.LastCommitState.LastBlockID) //TODO 这个blockId是否在正确
	case cmn.VoteType_Precommit:
		ourVotes = votes.Precommits(nrsMsg.Round).BitArrayByBlockID(h.LastCommitState.LastBlockID) //TODO 这个blockId是否在正确
	default:
		ConsLog.Errorf(LOGTABLE_CONS, "Bad VoteSetBitsMessage field Type")
		return nil
	}
	ConsLog.Infof(LOGTABLE_CONS, "%v", ourVotes)
	return nil
}
