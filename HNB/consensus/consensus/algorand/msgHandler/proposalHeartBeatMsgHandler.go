package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"encoding/json"
)

func (h *TDMMsgHandler) HandleProposalHeartBeatMsg(tdmMsg *cmn.TDMMessage) error {
	proposalHbMsg := &cmn.ProposalHeartbeatMessage{}

	err := json.Unmarshal(tdmMsg.Payload, proposalHbMsg)
	if err != nil {
		return err
	}

	return nil
}
