package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
)

func (h *TDMMsgHandler) HandleInnerVoteMsg(tdmMsg *cmn.TDMMessage, pubKeyID []byte) error {
	vote, err := h.buildTypeFromVoteMsg(tdmMsg)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "buildTypeFromVoteMsg Error vote:%v", vote)
	}
	err = h.tryAddVote(vote, pubKeyID)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "TryAddVote Error vote:%v", vote)
	}
	return err
}

func (h *TDMMsgHandler) HandleOutterVoteMsg(tdmMsg *cmn.TDMMessage, pubKeyID []byte) error {
	vote, err := h.buildTypeFromVoteMsg(tdmMsg)
	ConsLog.Infof(LOGTABLE_CONS,
		"#(%v-%v) (HandleOutterVoteMsg) From:%v  VoteMsg=(%v/%v/%v) vote=%v", h.Height, h.Round, vote.VoteString(), vote.Height, vote.Round, vote)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "buildTypeFromVoteMsg Error vote:%v", vote)
	}
	err = h.tryAddVote(vote, pubKeyID)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "TryAddVote Error vote:%v", vote)
	}
	return err
}
