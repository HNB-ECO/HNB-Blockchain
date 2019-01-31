package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"encoding/json"
)

func (h *TDMMsgHandler) HandleNewRoundStepMsg(tdmMsg *cmn.TDMMessage) error {
	nrsMsg := &cmn.NewRoundStepMessage{}
	ConsLog.Infof(LOGTABLE_CONS, "HandleNewRoundStepMsg  nrsMsg:", nrsMsg)
	err := json.Unmarshal(tdmMsg.Payload, nrsMsg)
	if err != nil {
		return err
	}

	return nil
}
