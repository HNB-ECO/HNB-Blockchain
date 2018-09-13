package msgHandler

import (
	"strconv"
	"fmt"
	"bytes"
	"sort"
	"HNB/consensus/algorand/types"
	cmn "HNB/consensus/algorand/common"
	"HNB/bccsp"
	"encoding/json"
	"errors"
	"strings"
	"HNB/ledger"
)

// 处理接收请求更换共识组列表消息
func (bftMgr *BftMgr) HandleBgDemand(tdmMsg *cmn.TDMMessage) (*BftGroupSwitchDemand, error) {
	bgDemand := &BftGroupSwitchDemand{}
	err := json.Unmarshal(tdmMsg.Payload, bgDemand)
	if err != nil {
		return nil, err
	}

	bgNum := bgDemand.BftGroupNum
	ConsLog.Infof(LOGTABLE_CONS, "receive bgDemand bgNum %d", bgNum)
	bgProposerAddr, err := bftMgr.GetNewBgProposerAddr(bgNum)
	if err != nil {
		return nil, err
	}
	// 需要是共识组列表发起人
	if !bytes.Equal(bgProposerAddr, bgDemand.DigestAddr) {
		return nil, fmt.Errorf("(bgDemand) bgProposer Addr %X != %x", bgProposerAddr, bgDemand.DigestAddr)
	}

	// 共识组编号需>当前共识组
	if bgNum < bftMgr.CurBftGroup.BgID {
		return nil, fmt.Errorf("(bgDemand) bgPropose bgNum %d != %d", bftMgr.CandidateID, bgNum)
	}

	//本命令是请求更换共识组列表，之前处理国请求更换共识组消息
	bgAdviceSlice := bftMgr.GetBgAdviceSlice(bgNum)
	// 需要收到超过1/3节点的请求更换共识组消息
	if len(bgAdviceSlice) <= len(bftMgr.TotalValidators.Validators)/3 {
		return nil, fmt.Errorf("(bgDemand) bgPropose bgNum %d not enough bgAdvice, receive %d <= %d", bgNum, len(bgAdviceSlice), len(bftMgr.TotalValidators.Validators)/3)
	}

	// 需要候选人数大于等于新共识组人数
	if len(bgDemand.BftGroupCandidates) < len(bgDemand.NewBftGroup) {
		return nil, fmt.Errorf("(bgDemand) candidate num %d < newBg Num %d", len(bgDemand.BftGroupCandidates), len(bgDemand.NewBftGroup))
	}

	// 新共识组人数等于指定bftNum
	if len(bgDemand.NewBftGroup) != int(bftMgr.BftNumber) {
		return nil, fmt.Errorf("(bgDemand) newBg num %d != %d", len(bgDemand.NewBftGroup), bftMgr.BftNumber)
	}

	//验证该共识组列表是否严格按照候选人方式选举
	newBg, err := bftMgr.MakeNewBgFromBgCandidateSlice(bgNum, bgDemand.BftGroupCandidates)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "make newBg err %v bgNum %d", err, bgNum)
		return nil, err
	}

	// 需要本地计算出的新共识组，等于发起人发送的
	if !bftMgr.NewBgEqualBCE(newBg, bgDemand.NewBftGroup) {
		return nil, fmt.Errorf("(bgDemand) bgPropose newBg %v != %v", newBg, bgDemand.NewBftGroup)
	}
	ConsLog.Infof(LOGTABLE_CONS, "wait got bgDemand bgNum %d <- %v",
		bgDemand.BftGroupNum,
			cmn.HexBytes(bgDemand.DigestAddr))

	newBftGroup, err := bftMgr.MakeNewBgFromBgAdvice(bgDemand.BftGroupNum, bgDemand.NewBftGroup)
	if err != nil {
		return nil, fmt.Errorf("(newBgEle) make newBftGroup err %v bgNum %d", err, bftMgr.CandidateID)
	}

	err = bftMgr.ResetBftGroup(*newBftGroup)
	if err != nil {
		return nil, fmt.Errorf("(newBgEle) reset bft group fail %v bgNum %d", err, bftMgr.CandidateID)
	}

	return bgDemand, nil
}

func (bftMgr *BftMgr) NewBgEqualBCE(newBg1, newBg2 []*BftGroupSwitchAdvice) bool {
	if len(newBg1) != len(newBg2) {
		return false
	}

	if (newBg1 == nil) != (newBg2 == nil) {
		return false
	}

	newBg2 = newBg2[0:len(newBg1)]
	for i := range newBg1 {
		if !bftMgr.AdviceEqual(newBg1[i], newBg2[i]) {
			return false
		}
	}

	return true
}

// 比较两个advice是否相等
func (bftMgr *BftMgr) AdviceEqual(advice1, advice2 *BftGroupSwitchAdvice) bool {
	if advice1.BftGroupNum != advice2.BftGroupNum {
		return false
	}

	if !bytes.Equal(advice1.DigestAddr, advice2.DigestAddr) {
		return false
	}

	if advice1.Height != advice2.Height {
		return false
	}

	if !bytes.Equal(advice1.Hash, advice2.Hash) {
		return false
	}

	return true
}

// 处理接收请求更换共识组消息
func (bftMgr *BftMgr) HandleBgAdviceMsg(tdmMsg *cmn.TDMMessage, senderID uint64) error {
	bgAdvice := &BftGroupSwitchAdvice{}
	err := json.Unmarshal(tdmMsg.Payload, bgAdvice)
	if err != nil {
		return err
	}

	bgNum := bgAdvice.BftGroupNum

	ConsLog.Infof(LOGTABLE_CONS, "receive bgNum %d advice <- %v", bgNum, cmn.HexBytes(bgAdvice.DigestAddr))
	if bgNum < bftMgr.CandidateID-1 {
		return fmt.Errorf("invalid bgAdvice %d < %d", bgNum, bftMgr.CandidateID)
	}

	if bgAdvice.Height > bftMgr.CurHeight {
		ConsLog.Infof(LOGTABLE_CONS, "receive h %d > %d sync", bgAdvice.Height, bftMgr.CurHeight)
		bftMgr.MsgHandler.SyncEntrance(bgAdvice.Height, senderID)
		return nil
	}

	// 验证hash
	hash, err :=ledger.GetBlockHash(bgAdvice.Height - 1)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS,
			"receive h %d > %d sync error: %v",
					bgAdvice.Height, bftMgr.CurHeight, err)
		return err
	}

	if !bytes.Equal(hash, bgAdvice.Hash) {
		ConsLog.Errorf(LOGTABLE_CONS,
			"invalid bgAdvice h %d hash %X != %X",
			bgAdvice.Height, bftMgr.CurHeight, err)
		return fmt.Errorf("invalid bgAdvice h %d hash %X != %X", bgAdvice.Height, bgAdvice.Hash, hash)
	}

	// 如果本节点已经发起bgDemand,不再接收bgAdvice
	if bftMgr.GetBgDemand(bgNum) != nil {
		return fmt.Errorf("bgAdvice already +2/3")
	}

	bftMgr.PutBgAdvice(bgAdvice)

	adviceSlice := bftMgr.GetBgAdviceSlice(bgNum)

	//当接收到更换共识组的命令已经超过1/3, 判断自身是否也发送更换命令
	if len(adviceSlice) > len(bftMgr.TotalValidators.Validators)/3 {
		ConsLog.Infof(LOGTABLE_CONS,
			"bgNum %d receive num > %d follow",
				bgNum, len(bftMgr.TotalValidators.Validators)/3)
		bgAdviceMsg, err := bftMgr.BuildBgAdvice(bftMgr.CurHeight, bgNum)
		if err != nil {
			return err
		}
		bftMgr.CandidateID = bgNum

		// 判断自己是否已经发出请求更换共识组
		if !bftMgr.HasBgAdvice(bftMgr.MsgHandler.digestAddr, bgNum) {
			ConsLog.Infof(LOGTABLE_CONS, "broadcast bgNum %d", bgNum)
			bftMgr.BroadcastMsgToAllVP(bgAdviceMsg)

			// 消息发给自己，统一处理
			select {
			case bftMgr.InternalMsgQueue <- bgAdviceMsg:
			default:
				ConsLog.Warningf(LOGTABLE_CONS, "recv msg chan full")
			}

		} else {
			ConsLog.Infof(LOGTABLE_CONS, "has broadcast bgAdvice bgNum %d", bgNum)
		}

	}

	//当超过2/3的节点发送更换共识组之后，则进行更换共识组列表操作
	if err := bftMgr.CheckResetBft(bgAdvice); err != nil {
		return err
	}

	return nil
}
