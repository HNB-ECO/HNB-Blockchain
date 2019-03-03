package msgHandler

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	appComm "github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr/common"
	tdm "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"math/rand"
)

const (
	TRY_TIMES_LIMIT int = 3
)

func PrintChcekFork(cf *tdm.CheckFork) string {
	return fmt.Sprintf("check frok msg bftGroupid [%d], height [%d], prehash [%s]",
		cf.BftGroupNum, cf.Height, hex.EncodeToString(cf.Hash))
}

func (h *TDMMsgHandler) broadCheckForkMsg() error {
	bftGroupid := h.getBftGroupID()
	height := h.Height
	preHash := h.LastCommitState.PreviousHash
	checkForkMsg := &tdm.CheckFork{
		BftGroupNum: bftGroupid,
		Height:      height,
		Hash:        preHash,
	}
	checkData, err := json.Marshal(checkForkMsg)
	if err != nil {
		return err
	}
	tdmMsg := &tdm.TDMMessage{
		Type:    tdm.TDMType_MsgCheckFork,
		Payload: checkData,
	}
	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		return err
	}
	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		return fmt.Errorf("marshal err [%s]", err.Error())
	}

	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return err
	}
	msg := reqMsg.NewConsMsg(conData)
	p2pNetwork.Xmit(msg, true)

	ConsLog.Infof(LOGTABLE_CONS, "success to broadcast %s", PrintChcekFork(checkForkMsg))
	return nil
}

func (h *TDMMsgHandler) HandleCheckFork(tdmMsg *tdm.TDMMessage, peerid uint64) error {
	if tdmMsg == nil {
		return fmt.Errorf("tdmMsg is nil")
	}

	checkFork := &tdm.CheckFork{}
	err := json.Unmarshal(tdmMsg.Payload, checkFork)
	if err != nil {
		return fmt.Errorf("unmarshal err [%s]", err.Error())
	}
	ConsLog.Infof(LOGTABLE_CONS, "recv [%s] from peer [%s]", PrintChcekFork(checkFork), peerid)
	myBgid := h.getBftGroupID()
	if myBgid >= checkFork.BftGroupNum {
		ConsLog.Infof(LOGTABLE_CONS, "BftGroupNum <= my")
		return nil
	}
	h.checkFork.clean(myBgid)
	ConsLog.Infof(LOGTABLE_CONS, "after clean")
	exists := h.checkFork.isExists(checkFork.BftGroupNum)
	if !exists {
		h.checkFork.initFork(checkFork.BftGroupNum)
	}
	ConsLog.Infof(LOGTABLE_CONS, "after checkExists")
	ok, err := h.checkFork.isDuplicate(checkFork.BftGroupNum, checkFork.Height, peerid)
	if err != nil {
		return err
	}
	if ok {
		//return nil
		ConsLog.Infof(LOGTABLE_CONS, "already has [%s] from peer [%s]", PrintChcekFork(checkFork), peerid)
	} else {
		peerHash := &PeerHash{
			peerid:  peerid,
			preHash: checkFork.Hash,
		}
		ConsLog.Infof(LOGTABLE_CONS, "save [%s] from peer [%s]", PrintChcekFork(checkFork), peerid)
		err = h.checkFork.addFork(checkFork.BftGroupNum, checkFork.Height, peerHash)
		if err != nil {
			return err
		}
		ConsLog.Infof(LOGTABLE_CONS, "after addFork")
	}
	allCount := len(h.getOtherVals()) + 1
	targetCount := 2*(allCount-1)/3 + 1 // 2f+1   (bft)
	ok, peers := h.checkFork.isCountToTarget(checkFork.BftGroupNum, checkFork.Height, checkFork.Hash, targetCount)
	if !ok {
		return nil
	}
	forkPoint, err := h.tryToSearchFork(peers)
	if err != nil {
		return err
	}
	curHeight, err := ledger.GetBlockHeight()
	if err != nil {
		return err
	}
	if forkPoint == curHeight-1 {
		ConsLog.Infof(LOGTABLE_CONS, "peer backward not fork")
		return nil
	}
	ConsLog.Infof(LOGTABLE_CONS, "find unforkPoint %d", forkPoint)
	err = h.rollBack(forkPoint+1, curHeight-1)
	if err != nil {
		return fmt.Errorf("rollback [%d->%d] err %s", forkPoint+1, h.Height-1, err.Error())
	}
	ConsLog.Infof(LOGTABLE_CONS, "success to  rollback [%d->%d]", forkPoint+1, h.Height-1)
	err = h.reloadLastBlock()
	if err != nil {
		return fmt.Errorf("reloadLastBlock err %s", err.Error())
	}
	randomIndex := rand.Intn(len(peers))
	peer := peers[randomIndex]

	ConsLog.Infof(LOGTABLE_CONS, "sync blk height [%d->%d] from %s", h.Height, checkFork.Height, peer)
	h.SyncEntrance(checkFork.Height, peer)
	return nil
}

func (h *TDMMsgHandler) tryToSearchFork(peers []uint64) (uint64, error) {
	tryCount := 0
	for tryCount <= TRY_TIMES_LIMIT {
		if !h.IsRunning() {
			return 0, fmt.Errorf("cons service msgHandler has been stopped")
		}
		tryCount++
		randomIndex := rand.Intn(len(peers))
		peerid := peers[randomIndex]

		forkPoint, err := h.forkSearchWorker.SearchFork(peerid, appComm.HNB)
		if err != nil {
			ConsLog.Warningf(LOGTABLE_CONS, "SearchFork err [%s]", err.Error())
		} else {
			return forkPoint, nil
		}

	}
	return 0, fmt.Errorf("can not find fork point")
}
