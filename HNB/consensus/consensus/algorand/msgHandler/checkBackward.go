package msgHandler

import (
	"HNB/p2pNetwork/message/reqMsg"
	"encoding/json"
	"fmt"
	"time"

	com "HNB/consensus/algorand/common"
	"HNB/consensus/consensusManager/comm/consensusType"
	"HNB/ledger"
	"HNB/p2pNetwork"
	"math/rand"
)

const (
	NORMAL int = iota + 1
	VERSION_ERRO
	HEIGHT_ERRO
	PARA_NIL_ERRO
)

type HeightReqOutBg struct {
	version uint64
}

func NewHeightReqOutBG() *HeightReqOutBg {
	return &HeightReqOutBg{}
}

type HeightReq struct {
	version uint64
	min     *MinHeightPeer
	count   int
}

type MinHeightPeer struct {
	peerID uint64
	blkNum uint64
}

func NewHeightReq() *HeightReq {
	return &HeightReq{min: &MinHeightPeer{}}
}

func (h *TDMMsgHandler) SetMinHeightPeer(peerID uint64, blkNum uint64) {
	if peerID < 0 || blkNum < 0 {
		ConsLog.Errorf(CONSENSUS, "(setminheightpeer) peerID (%v)   blkNum (%d)", peerID, blkNum)
		return
	}
	h.heightReq.min.peerID = peerID
	h.heightReq.min.blkNum = blkNum
}

func (h *TDMMsgHandler) ResetMinHeightPeer() {
	h.heightReq.version++
	h.heightReq.min.peerID = 0
	h.heightReq.min.blkNum = 0
	h.heightReq.count = 0
}

func (h *TDMMsgHandler) GetMinHeightPeer() (uint64, uint64, error) {
	peerID := h.heightReq.min.peerID
	blkNum := h.heightReq.min.blkNum
	return peerID, blkNum, nil
}

func (h *TDMMsgHandler) Monitor() {
	h.allRoutineExitWg.Add(1)
	tick := time.NewTicker(3000 * time.Millisecond)
	for {
		select {
		case <-tick.C:
			ConsLog.Infof(CONSENSUS, "tick time out")
			h.ResetMinHeightPeer() //timeout to rest the target min peer witch bloknum biger than me (peer count must +2/3)
			h.checkBackwardFromBg()

		case <-h.Quit():
			ConsLog.Infof(CONSENSUS, "(msgHandler) sync req stopped")
			h.allRoutineExitWg.Done()
			return
		}
	}
}

func (h *TDMMsgHandler) getOtherPeers() ([]uint64, error) {
	peersInfo := p2pNetwork.GetNeighborAddrs()

	retPeerEndPointSlice := make([]uint64, 0)
	for _, peerEndPoint := range peersInfo {
		retPeerEndPointSlice = append(retPeerEndPointSlice, peerEndPoint.ID)
	}

	return retPeerEndPointSlice, nil
}

func (h *TDMMsgHandler) checkBackwardFromBg() {
	if h.isSyncStatus.Get() {
		ConsLog.Warningf(CONSENSUS, "already sync")
		return
	}

	otherVals := p2pNetwork.GetNeighborAddrs()
	if len(otherVals) == 0 {
		return
	}
	allCount := len(otherVals) + 1
	targetCount := (allCount-1)/3 + 1 // f+1   (bft)
	targetCount = 1
	peerCount := len(otherVals)
	list := make(map[int]struct{}, targetCount)
	tryTimes := 0
	for len(list) < targetCount && peerCount >= targetCount && tryTimes <= 3 {
		tryTimes++
		ConsLog.Debugf(CONSENSUS, "list len %d", len(list))
		randomIndex := rand.Intn(len(otherVals))
		_, ok := list[randomIndex]
		if ok {
			continue
		}
		val := otherVals[randomIndex]
		peerID := val.ID

		if peerID < 0 {
			ConsLog.Warningf(CONSENSUS, "not available peer")
			time.Sleep(1 * time.Second) //choose peer unavailable, sleep 1s to avoid abnormal loop logging
		} else {
			err := h.BlockHeightReqFromPeer(peerID, true)
			if err != nil {
				ConsLog.Warningf(CONSENSUS, "send blk req msg err [%s]", err.Error())
				time.Sleep(1 * time.Second) //"send blk req msg err, sleep 1s to avoid abnormal loop logging
			} else {
				list[randomIndex] = struct{}{}
			}
		}
	}

}

func (h *TDMMsgHandler) BlockHeightReqFromPeer(peeID uint64, isFromBg bool) error {

	var version uint64
	if isFromBg {
		version = h.heightReq.version
	} else {
		version = h.heightReqOutBg.version
	}

	heightReq := &com.HeightReq{
		Version:  version,
		IsFromBg: isFromBg,
	}
	repData, err := json.Marshal(heightReq)
	if err != nil {
		return err
	}

	tdmMsg := &com.TDMMessage{
		Type:    com.TDMType_MsgHeightReq,
		Payload: repData,
	}
	tdmMsgSign, err := h.Sign(tdmMsg)
	if err != nil {
		return err
	}

	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		return fmt.Errorf("marshal err [%s]", err)
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return err
	}

	data := &com.PeerMessage{
		Sender:      h.ID,
		Msg:         conData,
		IsBroadCast: false,
	}
	msg, _ := json.Marshal(data)
	p2pNetwork.Send(peeID, reqMsg.NewConsMsg(msg), true)
	ConsLog.Infof(CONSENSUS, "unicast blk height req succ to [%v]", peeID)
	return nil
}

func (h *TDMMsgHandler) SendBlockHeightReqToPeer(tdmMsg *com.TDMMessage, peeID uint64) error {
	if tdmMsg == nil {
		return fmt.Errorf("tdmMsg is nil")
	}
	if peeID < 0 {
		return fmt.Errorf("peer is nil")
	}

	heightReq := &com.HeightReq{}
	err := json.Unmarshal(tdmMsg.Payload, heightReq)
	if err != nil {
		return fmt.Errorf("unmarshal err [%s]", err.Error())
	}

	height, err := ledger.GetBlockHeight()
	if err != nil {
		return fmt.Errorf("getblockheight err [%s]", err.Error())
	}
	blk, err := ledger.GetBlock(height - 1)
	if err != nil {
		return fmt.Errorf("getblock err [%s]", err.Error())
	}
	if blk == nil {
		return fmt.Errorf("blk is nil")
	}
	respMsg := &com.HeightResp{
		BlockNum: height - 1,
		Header:   blk.Header,
		Version:  heightReq.Version,
		IsFromBg: heightReq.IsFromBg,
	}
	respData, err := json.Marshal(respMsg)
	if err != nil {
		return err
	}
	tdmMsgData := &com.TDMMessage{
		Type:    com.TDMType_MsgHeihtResp,
		Payload: respData,
	}
	tdmMsgSign, err := h.Sign(tdmMsgData)
	if err != nil {
		return err
	}

	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		return err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return err
	}
	data := &com.PeerMessage{
		Sender:      h.ID,
		Msg:         conData,
		IsBroadCast: false,
	}

	msg, _ := json.Marshal(data)
	p2pNetwork.Send(peeID, reqMsg.NewConsMsg(msg), true)
	ConsLog.Infof(CONSENSUS, "unicast blk height resp succ to [%v]", peeID)
	return nil
}

func (h *TDMMsgHandler) ReciveHeihtResp(tdmMsg *com.TDMMessage, peeID uint64) error {
	if tdmMsg == nil {
		return fmt.Errorf("tdmMsg is nil")
	}

	heightResp := &com.HeightResp{}
	err := json.Unmarshal(tdmMsg.Payload, heightResp)
	if err != nil {
		return fmt.Errorf("unmarshal err [%s]", err.Error())
	}
	typ, err := h.VerifHeihtRespAll(heightResp)
	if err != nil {
		if typ != HEIGHT_ERRO {
			ConsLog.Warningf(CONSENSUS, "verif failed [%s]", err.Error())
		}
		return nil
	}
	if heightResp.IsFromBg {
		h.heightReq.count++
		height := h.Height
		peerID, blkNum, err := h.GetMinHeightPeer()
		if err != nil {
			return fmt.Errorf("getminheightpeer err [%s]", err.Error())
		}

		if heightResp.BlockNum < blkNum || blkNum == 0 {
			h.SetMinHeightPeer(peeID, heightResp.BlockNum)
			peerID = peeID
			blkNum = heightResp.BlockNum
		}
		ok := h.checkHeightRespCount()
		if ok {
			if height < blkNum+1 {
				ConsLog.Infof(CONSENSUS, "(sync blk) backward local height [%d] remote height [%d] <-%v", height, heightResp.BlockNum+1, peerID)
				h.SyncEntrance(heightResp.BlockNum+1, peerID)
			}
		}
	} else {
		if h.Height < heightResp.BlockNum+1 {
			ConsLog.Infof(CONSENSUS, "(sync blk) OutBg backward local height [%d] remote height [%d] <-%v", h.Height, heightResp.BlockNum+1, peeID)
			h.SyncEntrance(heightResp.BlockNum+1, peeID)
		}
	}

	return nil
}

func (h *TDMMsgHandler) checkHeightRespCount() bool {
	allCount := len(h.getOtherVals()) + 1
	targetCount := (allCount-1)/3 + 1 // f+1   (bft)
	targetCount = 1                   // todo   这里先改成收到一个就先进行同步
	ConsLog.Debugf(CONSENSUS, "heightresp now (%d) target (%d)", h.heightReq.count, targetCount)
	if h.heightReq.count >= targetCount {
		return true
	}
	return false
}

func (h *TDMMsgHandler) VerifHeihtRespAll(heightResp *com.HeightResp) (int, error) {
	if heightResp == nil {
		return PARA_NIL_ERRO, fmt.Errorf("heightResp is nil")
	}
	if heightResp.IsFromBg {
		return h.VerifHeihtResp(heightResp)
	} else {
		return h.VerifHeihtRespOutBG(heightResp)
	}
}

func (h *TDMMsgHandler) VerifHeihtResp(heightResp *com.HeightResp) (int, error) {

	if heightResp.Version != h.heightReq.version {
		return VERSION_ERRO, fmt.Errorf("Resp version (%d) not match version (%d)", heightResp.Version, h.heightReq.version)
	}
	if heightResp.BlockNum+1 <= h.Height {
		return HEIGHT_ERRO, fmt.Errorf("Resp blkheight (%d) <= my blkblkheightnum (%d)", heightResp.BlockNum+1, h.Height)
	}
	return NORMAL, nil
}

func (h *TDMMsgHandler) VerifHeihtRespOutBG(heightResp *com.HeightResp) (int, error) {
	if heightResp.Version != h.heightReqOutBg.version {
		return VERSION_ERRO, fmt.Errorf("Resp OutBg version (%d) not match version (%d)", heightResp.Version, h.heightReqOutBg.version)
	}
	if heightResp.BlockNum+1 <= h.Height {
		return HEIGHT_ERRO, fmt.Errorf("Resp OutBg blkheight (%d) <= my blkblkheightnum (%d)", heightResp.BlockNum+1, h.Height)
	}
	return NORMAL, nil
}
