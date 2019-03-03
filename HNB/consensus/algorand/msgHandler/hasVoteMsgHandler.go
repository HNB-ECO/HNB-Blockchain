package msgHandler

import (
	"encoding/json"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
)

func (h *TDMMsgHandler) HandleHasVoteMsg(tdmMsg *cmn.TDMMessage) error {
	h.mtx.Lock()
	nrsMsg := &cmn.HasVoteMessage{}
	err := json.Unmarshal(tdmMsg.Payload, nrsMsg)
	if err != nil {
		return err
	}
	defer h.mtx.Unlock()
	if h.Height != nrsMsg.Height {
		return nil
	}
	return nil
}
