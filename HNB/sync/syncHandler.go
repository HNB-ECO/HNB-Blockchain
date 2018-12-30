package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	pbLedger "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/blockStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	netCommon "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	mt "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/bean"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	syncComm "github.com/HNB-ECO/HNB-Blockchain/HNB/sync/common"
	"math/rand"
	"time"
)

func getBlocksScope(blocks []*pbLedger.Block) (uint64, uint64) {
	if blocks == nil {
		return 0, 0
	} else {
		length := len(blocks)
		return blocks[0].Header.BlockNum, blocks[length-1].Header.BlockNum
	}

}

func (sh *SyncHandler) processRequestBlocks(req *syncComm.RequestBlocks, peerID uint64) {

	res := &syncComm.ResponseBlocks{}

	length := req.EndIndex - req.BeginIndex + 1
	blocks := make([]*pbLedger.Block, length)
	headers := make([]uint64, len(blocks))

	for i := req.BeginIndex; i <= req.EndIndex; i++ {
		block, err := ledger.GetBlock(i)
		if block == nil || err != nil {
			syncLogger.Warningf(LOGTABLE_SYNC, "chainID(%s).(sync recv req blks(%d,%d) v(%v)).(refuse)",
				req.ChainID,
				req.BeginIndex,
				req.EndIndex,
				req.Version)
			blocks = nil
			break
		}
		blocks[i-req.BeginIndex] = block
		headers[i-req.BeginIndex] = block.Header.BlockNum
	}

	res.ChainID = req.ChainID
	res.Blocks = blocks
	res.Version = req.Version

	payload, err := json.Marshal(res)
	if err != nil {
		syncLogger.Warningf(LOGTABLE_SYNC, err.Error())
		return
	}

	syncMsg := &syncComm.SyncMessage{
		Type:    syncComm.SyncEventType_RESPONSE_BLOCKS_EVENT,
		Payload: payload,
		Sender:  peerID,
	}

	payload, err = json.Marshal(syncMsg)

	if err != nil {
		return
	}
	msg := reqMsg.NewSyncMsg(payload)
	//msg := &pbNetwork.Message{
	//	Type:    pbNetwork.Message_SYNC_BLOCKS,
	//	Payload: payload,
	//}

	syncLogger.Infof(LOGTABLE_SYNC, "* (sync req).(res blks(%d, %d) v(%d) -> %v)",
		req.BeginIndex,
		req.EndIndex,
		req.Version,
		peerID)

	syncLogger.Debugf(LOGTABLE_SYNC, "* (sync req) res blks len=%d", len(blocks), headers)

	p2pNetwork.Send(peerID, msg, false)
}

func (sh *SyncHandler) HandlerMessage(msg []byte, msgSender uint64) {

	syncMsg := &syncComm.SyncMessage{}
	err := json.Unmarshal(msg, syncMsg)
	if err != nil {
		syncLogger.Infof(LOGTABLE_SYNC, "unmar %s", string(msg))
		syncLogger.Errorf(LOGTABLE_SYNC, "(sync req).(req unmar).(%s)", err.Error())
		return
	}

	switch syncMsg.Type {
	case syncComm.SyncEventType_REQUEST_BLOCKS_EVENT:
		req := &syncComm.RequestBlocks{}
		err := json.Unmarshal(syncMsg.Payload, req)
		if err != nil {
			syncLogger.Errorf(LOGTABLE_SYNC, "(sync req).(req unmar).(%s)", err.Error())
			return
		}

		syncLogger.Infof(LOGTABLE_SYNC, "* (sync req).(%v -> recv req blks(%d, %d), v(%d)).(parsing)",
			syncMsg.Sender,
			req.BeginIndex,
			req.EndIndex,
			req.Version)

		sh.processRequestBlocks(req, syncMsg.Sender)

	case syncComm.SyncEventType_RESPONSE_BLOCKS_EVENT:

		res := &syncComm.ResponseBlocks{}
		err := json.Unmarshal(syncMsg.Payload, res)
		if err != nil {
			syncLogger.Errorf(LOGTABLE_SYNC, "(sync res).(unmar).(%s)", err.Error())
			return
		}

		sch := sh.getSyncHandlerByChainID(res.ChainID)
		if sch != nil {
			if (sch.getSyncVersion() - 1) == res.Version {
				sch.syncBlockChannel <- res
			} else {
				syncLogger.Warningf(LOGTABLE_SYNC, "* (sync res).(version %d != %d)",
					(sch.getSyncVersion() - 1), res.Version)
			}
		}
	}
	//return nil
}

func (sch *syncChainHandler) genRemoteBlocksRequest(beginIndex uint64, endIndex uint64) (*mt.SyncMsg, error) {
	version := sch.getSyncVersion()
	reqBlock := &syncComm.RequestBlocks{
		ChainID:    sch.chainID,
		BeginIndex: beginIndex,
		EndIndex:   endIndex,
		Version:    version,
	}

	syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(sub sync blks (%d,%d) v(%d))",
		sch.chainID,
		beginIndex,
		endIndex,
		version)

	sch.setSyncVersion((version + uint64(1)))

	payload, err := json.Marshal(reqBlock)

	if err != nil {
		syncLogger.Errorf(LOGTABLE_SYNC, "chainID(%s).(unmar sync req).(%s)", sch.chainID, err.Error())
		return nil, err
	}

	syncMsg := &syncComm.SyncMessage{
		Type:    syncComm.SyncEventType_REQUEST_BLOCKS_EVENT,
		Payload: payload,
		Sender:  p2pNetwork.GetLocatePeerID(),
	}

	payload, err = json.Marshal(syncMsg)

	if err != nil {
		syncLogger.Errorf(LOGTABLE_SYNC, "chainID(%s).(unmar sync req).(%s)", sch.chainID, err.Error())
		return nil, err
	}
	msg := reqMsg.NewSyncMsg(payload)
	//msg := &pbNetwork.Message{
	//	Type:    pbNetwork.Message_SYNC_BLOCKS,
	//	Payload: payload,
	//}

	return msg, nil
}

func (sch *syncChainHandler) GetInterval(peerID uint64) (int, error) {
	sch.valLock.RLock()
	defer sch.valLock.RUnlock()

	v, ok := sch.interval[peerID]
	if !ok {
		return 0, errors.New("not exist")
	}

	return v, nil
}

func (sch *syncChainHandler) SetInterval(peerID uint64, interval int) {
	sch.valLock.Lock()
	sch.interval[peerID] = interval
	sch.valLock.Unlock()
}

func (sch *syncChainHandler) getRemoteBlocks(beginIndex uint64, endIndex uint64, peerID uint64) ([]*pbLedger.Block, error) {

	//nw := sch.sh.nw
	//localEndpoint, _ := nw.GetPeerEndpoint()
	localPeerID := p2pNetwork.GetLocatePeerID()
	var retransmissions int8 = 0
	var reSendMessage bool = true

	var tmpBeginIndex, tmpEndIndex uint64

	tmpBeginIndex = beginIndex
	tmpEndIndex = endIndex

	for {
		if reSendMessage == true {
			var N int
			var peers []netCommon.PeerAddr

			msg, err := sch.genRemoteBlocksRequest(tmpBeginIndex, tmpEndIndex)
			if err != nil {
				return nil, err
			}

			for {
				peers = p2pNetwork.GetNeighborAddrs()
				//peers, err = sch.sh.nw.GetPeers()
				//if err != nil {
				//	syncLogger.Warningf(LOGTABLE_SYNC,"chainID(%s).(sync get peers).(%s)", sch.chainID, err.Error())
				//	continue
				//}
				N = len(peers)

				if N != 0 {
					break
				} else {
					syncLogger.Infof(LOGTABLE_SYNC, "waiting network init over")
					time.Sleep(3 * time.Second)
				}
			}

			if peerID < 0 {
				ipIndex := rand.Int() % N

				if localPeerID == peers[ipIndex].ID {
					continue
				}

				syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(send sync req(%d,%d) => %v)",
					sch.chainID,
					tmpBeginIndex,
					tmpEndIndex,
					peers[ipIndex].ID)
				p2pNetwork.Send(peers[ipIndex].ID, msg, false)
				//nw.Unicast(msg, peers.Peers[ipIndex].PeerID)

			} else {
				syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(send sync req(%d,%d) => %v times(%d/%d))",
					sch.chainID,
					tmpBeginIndex,
					tmpEndIndex,
					peerID,
					retransmissions,
					5)
				p2pNetwork.Send(peerID, msg, false)
				//nw.Unicast(msg, peerID)
				retransmissions++
			}

		}
		select {
		case <-sch.exitTask:
			sch.setSyncVersion((sch.getSyncVersion() + uint64(1)))
			syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(cons notify stop sync)", sch.chainID)
			return nil, errors.New("stop sync")
		default:
		}

		select {
		case resBlock := <-sch.syncBlockChannel:
			blocks := resBlock.GetBlocks()
			if blocks == nil {
				syncLogger.Warningf(LOGTABLE_SYNC, "* chainID(%s).(recv sync blk(%d,%d) = nil",
					sch.chainID,
					tmpBeginIndex,
					tmpEndIndex)
				return nil, nil
			}

			recvBeginIndex, recvEndIndex := getBlocksScope(blocks)
			syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(recv sync blk(%d,%d))",
				sch.chainID,
				recvBeginIndex,
				recvEndIndex)

			if (recvBeginIndex != tmpBeginIndex) || (recvEndIndex != tmpEndIndex) {
				syncLogger.Warningf(LOGTABLE_SYNC, "* chainID(%s).(recv sync blk(%d,%d) != (%d,%d))",
					sch.chainID,
					tmpBeginIndex,
					tmpEndIndex,
					recvBeginIndex,
					recvEndIndex)
				reSendMessage = true
				continue
			} else {
				return blocks, nil
			}

		case <-time.After(time.Duration(sch.timeout) * time.Second):
			reSendMessage = true

			if retransmissions == 1 || retransmissions == 3 {
				v, _ := sch.GetInterval(peerID)
				interval := v / 2
				if interval < 1 {
					interval = 1
				}
				syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(set interval(%v))", sch.chainID, interval)
				sch.SetInterval(peerID, interval)
				end := tmpEndIndex
				tmpEndIndex = tmpBeginIndex + uint64(interval) - 1
				syncLogger.Infof(LOGTABLE_SYNC, "* chainID(%s).(change sync(%d,%d) -> (%d,%d))",
					sch.chainID,
					tmpBeginIndex, end,
					tmpBeginIndex, tmpEndIndex)
			}

			if retransmissions == 5 {
				errInfo := fmt.Sprintf("* chainID(%s).(retransmissions timeout)", sch.chainID)
				syncLogger.Warningf(LOGTABLE_SYNC, errInfo)
				return nil, errors.New(errInfo)
			}
			continue

		}

	}
	return nil, nil
}

func (sch *syncChainHandler) getSyncVersion() uint64 {
	return sch.chainSyncVersion
}

func (sch *syncChainHandler) setSyncVersion(version uint64) {
	sch.chainSyncVersion = version
}
