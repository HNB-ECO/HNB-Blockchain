package msgHandler

import (
	tendermint "HNB/consensus/algorand/common"
	"HNB/p2pNetwork"
	"HNB/p2pNetwork/message/reqMsg"
	"HNB/util"
	"encoding/json"
	"github.com/op/go-logging"

	"sync"

	"time"

	"math"

	"errors"

	"bytes"

	"fmt"

	"HNB/consensus/consensusManager/comm/consensusType"
	"HNB/ledger"
)

type ForkSearchWorker struct {
	ResChan chan *tendermint.TDMMessage
	jobList map[string][]uint64
	sync.RWMutex
	jobLocker sync.RWMutex
	Logger    *logging.Logger
}

const (
	BINARY_SEARCH_BASE_COUNT_DEF       = 512
	BINARY_SEARCH_TIMEOUT_DEF          = time.Second * 30
	BINARY_SEARCH_HEIGHT_TOLERANCE_DEF = 50
)

func NewForkSearchWorker() (*ForkSearchWorker, error) {

	fsWorker := &ForkSearchWorker{
		ResChan: make(chan *tendermint.TDMMessage, 10),
		jobList: make(map[string][]uint64, 10),
	}

	return fsWorker, nil
}

func (fsWorker *ForkSearchWorker) SearchFork(peerId uint64, chainId string) (forkPoint uint64, err error) {
	fsWorker.jobLocker.Lock()
	defer fsWorker.jobLocker.Unlock()

	//发送消息
	reqId := util.GenerateBytesUUID()
	req, err := fsWorker.GenBinaryBlockHashReq(reqId, chainId)
	if err != nil {
		return 0, errors.New("get local height err")
	}

	if len(req.BlockNumSlice) == 0 {
		return 1, nil
	}

	fsWorker.SendReq(peerId, req)

	forkPoint, err = fsWorker.HandleResBlkHash(reqId, chainId, peerId)
	fsWorker.RemoveJob(reqId)
	return forkPoint, err

}

func (fsWorker *ForkSearchWorker) HandleResBlkHash(reqId, chainId string, peerId uint64) (forkPoint uint64, err error) {

	var timeout time.Duration = BINARY_SEARCH_TIMEOUT_DEF

	forkSearchTimeoutConf := 30
	if forkSearchTimeoutConf != 0 {
		timeout = time.Duration(forkSearchTimeoutConf) * time.Second
	}

	fsWorker.Logger.Infof("forkSearch job %v set timeLimit %v", reqId, timeout)

	timer := time.NewTimer(timeout)

	for {
		select {
		case blkHashResMsg := <-fsWorker.ResChan:
			blkHashResp := &tendermint.BinaryBlockHashResp{}
			if err := json.Unmarshal(blkHashResMsg.Payload, blkHashResp); err != nil {
				fsWorker.Logger.Errorf("forkSearch blkHashResp unmarshal err")
				return 0, fmt.Errorf("forkSearch blkHashResp unmarshal err")
			}

			fsWorker.Logger.Infof("forkSearch job %v recv resp", blkHashResp.ReqId)

			reqBlkNumSlice := fsWorker.GetJob(blkHashResp.GetReqId())
			if len(reqBlkNumSlice) == 0 {
				fsWorker.Logger.Warningf("forkSearch job %s not exist", blkHashResp.GetReqId())
				continue
			}

			fsWorker.Logger.Infof("forkSearch job %v resp [%d,%d]", blkHashResp.ReqId, reqBlkNumSlice[0], reqBlkNumSlice[len(reqBlkNumSlice)-1])

			leftPointValue, rightPointValue, err := fsWorker.GetForkPointInterval(chainId, reqBlkNumSlice, blkHashResp)
			if err != nil {
				return 0, err
			}

			if leftPointValue == 0 {
				leftPointValue += 1
			}

			if leftPointValue == 1 && rightPointValue == 1 {
				fsWorker.Logger.Warningf("forkSearch job %s forkPoint==1", blkHashResp.GetReqId())
				return 1, nil
			}

			if (rightPointValue - leftPointValue) < BINARY_SEARCH_HEIGHT_TOLERANCE_DEF {
				fsWorker.Logger.Warningf("forkSearch job %s forkPoint==%v", blkHashResp.GetReqId(), leftPointValue)
				return leftPointValue, nil
			}

			req, err := fsWorker.ReGenBinaryBlockHashReq(blkHashResp.GetReqId(), chainId, leftPointValue, rightPointValue)
			if err != nil {
				fsWorker.Logger.Errorf("forkSearch ReGenBinaryBlockHashReq err %v", err)
				return 0, err
			}

			err = fsWorker.SendReq(peerId, req)
			if err != nil {
				fsWorker.Logger.Errorf("forkSearch SendReq err %v", err)
				return 0, err
			}

			timer.Reset(BINARY_SEARCH_TIMEOUT_DEF)
		case <-timer.C:
			fsWorker.Logger.Warningf("forkSearch job %v timeout", reqId)

			return 0, fmt.Errorf("forkSearch job %v timeout", reqId)
		}
	}
}

func (fsWorker *ForkSearchWorker) GetForkPointInterval(chainId string, reqBlkNumSlice []uint64, blkHashResp *tendermint.BinaryBlockHashResp) (uint64, uint64, error) {
	var i int

	var leftPointValue uint64 = 1
	var rightPointValue uint64 = 1
	for i = len(reqBlkNumSlice) - 1; i >= 0; i-- {
		blkNum := reqBlkNumSlice[i]
		localHash, err := ledger.GetBlockHash(blkNum)
		if err != nil {
			fsWorker.Logger.Warningf("forkSearch local blk=%v not exist", blkNum)
			continue
		}

		respHash := blkHashResp.GetBlockHashSlice()[i]
		if respHash == nil ||
			len(respHash) == 0 {
			fsWorker.Logger.Warningf("forkSearch resp blk=%v hash empty", blkNum)
			continue
		}

		hashEqual := bytes.Equal(localHash, respHash)
		if hashEqual {
			leftPointValue = blkNum
			if i == (len(reqBlkNumSlice) - 1) {
				rightPointValue = leftPointValue
			} else {
				rightPointValue = reqBlkNumSlice[i+1]
			}

			break
		}
	}

	return leftPointValue, rightPointValue, nil
}

func (fsWorker *ForkSearchWorker) SendReq(peerId uint64, msg *tendermint.BinaryBlockHashReq) error {
	reqData, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	tendermintMsg := &tendermint.TDMMessage{Type: tendermint.TDMType_MsgBinaryBlockHashReq, Payload: reqData}
	consensusData, err := json.Marshal(tendermintMsg)
	if err != nil {
		return err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: consensusData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return err
	}

	p2pNetwork.Send(peerId, reqMsg.NewConsMsg(conData), true)

	fsWorker.PutJob(msg.GetReqId(), msg.BlockNumSlice)
	fsWorker.Logger.Infof("forkSearch job %v req [%d,%d]", msg.ReqId, msg.BlockNumSlice[0], msg.BlockNumSlice[len(msg.BlockNumSlice)-1])
	return nil
}

func (fsWorker *ForkSearchWorker) ReGenBinaryBlockHashReq(reqId, chainId string, leftPointValue uint64, rightPointValue uint64) (blkHashReq *tendermint.BinaryBlockHashReq, err error) {
	reqBlockNumSlice := fsWorker.GenBinSearchBaseCountBlkNum(leftPointValue+1, rightPointValue-1)
	blkHashReq = &tendermint.BinaryBlockHashReq{ReqId: reqId, ChainId: chainId, BlockNumSlice: reqBlockNumSlice}

	return blkHashReq, nil
}

func (fsWorker *ForkSearchWorker) GenBinaryBlockHashReq(reqId, chainId string) (blkHashReq *tendermint.BinaryBlockHashReq, err error) {
	localHeight, err := ledger.GetBlockHeight()
	if err != nil {
		return nil, err
	}

	curBlkNum := localHeight - 1

	reqBlockNumSlice := fsWorker.GenBinSearchBaseCountBlkNum(1, curBlkNum)

	blkHashReq = &tendermint.BinaryBlockHashReq{ReqId: reqId, ChainId: chainId, BlockNumSlice: reqBlockNumSlice}
	return blkHashReq, nil
}

func (fsWorker *ForkSearchWorker) GenBinSearchBaseCountBlkNum(startBlkNum, endBlkNum uint64) []uint64 {
	if startBlkNum > endBlkNum {
		var reqBlockNumSlice []uint64 = make([]uint64, 0)
		return reqBlockNumSlice
	}

	var reqBlockNumSlice []uint64
	counts := (endBlkNum - startBlkNum) + 1
	if counts < BINARY_SEARCH_BASE_COUNT_DEF {
		var i uint64
		reqBlockNumSlice = make([]uint64, counts)
		for i = 0; i < counts; i++ {
			reqBlockNumSlice[i] = startBlkNum + i
		}

	} else {
		reqBlockNumSlice = make([]uint64, BINARY_SEARCH_BASE_COUNT_DEF+1)
		var interval float64 = float64(endBlkNum-startBlkNum) / BINARY_SEARCH_BASE_COUNT_DEF
		var i uint64
		for i = 0; i <= BINARY_SEARCH_BASE_COUNT_DEF; i++ {
			reqBlockNumSlice[i] = uint64(math.Floor(float64(startBlkNum) + float64(i)*interval))
		}
	}

	return reqBlockNumSlice
}

func (fsWorker *ForkSearchWorker) SendBinaryBlockHashResp(blkHashReqMsg *tendermint.TDMMessage, peerId uint64) error {
	hashReq := &tendermint.BinaryBlockHashReq{}
	if err := json.Unmarshal(blkHashReqMsg.Payload, hashReq); err != nil {
		return err
	}

	resp, err := fsWorker.GenBinaryBlockHashResp(hashReq)
	if err != nil {
		return err
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	tendermintMsg := &tendermint.TDMMessage{Type: tendermint.TDMType_MsgBinaryBlockHashResp, Payload: respData}
	consensusData, err := json.Marshal(tendermintMsg)
	if err != nil {
		return err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: consensusData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return err
	}

	p2pNetwork.Send(peerId, reqMsg.NewConsMsg(conData), true)

	return nil
}

func (fsWorker *ForkSearchWorker) GenBinaryBlockHashResp(hashReq *tendermint.BinaryBlockHashReq) (*tendermint.BinaryBlockHashResp, error) {

	reqBlkNumSlice := hashReq.GetBlockNumSlice()
	var hashSlice [][]byte = make([][]byte, len(reqBlkNumSlice))
	for index, blkNum := range reqBlkNumSlice {
		hash, err := ledger.GetBlockHash(blkNum)
		if err == nil {
			hashSlice[index] = hash
		}
	}

	resp := &tendermint.BinaryBlockHashResp{
		ReqId:          hashReq.GetReqId(),
		BlockHashSlice: hashSlice}

	return resp, nil
}

func (fsWorker *ForkSearchWorker) PutJob(reqId string, reqBlkNumSlice []uint64) {
	fsWorker.Lock()
	defer fsWorker.Unlock()

	fsWorker.jobList[reqId] = reqBlkNumSlice
}

func (fsWorker *ForkSearchWorker) GetJob(reqId string) []uint64 {
	fsWorker.RLock()
	defer fsWorker.RUnlock()

	if blkNumSlice, ok := fsWorker.jobList[reqId]; ok {
		return blkNumSlice
	}

	return make([]uint64, 0)
}

func (fsWorker *ForkSearchWorker) RemoveJob(reqId string) {
	fsWorker.RLock()
	defer fsWorker.RUnlock()

	if _, ok := fsWorker.jobList[reqId]; ok {
		delete(fsWorker.jobList, reqId)
	}
}
