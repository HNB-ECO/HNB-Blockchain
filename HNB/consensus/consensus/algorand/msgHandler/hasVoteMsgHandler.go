package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"encoding/json"
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
