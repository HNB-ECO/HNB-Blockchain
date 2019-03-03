package msgHandler

import (
	"encoding/json"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
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
