package msgHandler

import (
	"encoding/json"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
)

func (h *TDMMsgHandler) HandleProposalHeartBeatMsg(tdmMsg *cmn.TDMMessage) error {
	proposalHbMsg := &cmn.ProposalHeartbeatMessage{}

	err := json.Unmarshal(tdmMsg.Payload, proposalHbMsg)
	if err != nil {
		return err
	}

	return nil
}
