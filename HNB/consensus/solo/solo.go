package solo

import (
	"HNB/appMgr"
	"HNB/config"
	"HNB/ledger"
	blkCom "HNB/ledger/blockStore/common"
	"HNB/logging"
	"HNB/p2pNetwork"
	"HNB/p2pNetwork/message/reqMsg"
	"HNB/txpool"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"time"
)

var SoloLog logging.LogModule

const (
	LOGTABLE_SOLO = "solo"
)

type SoloServer struct {
	chainId     string
	blkNum      uint64
	emptyEnable bool
	blkTimer    *time.Ticker
	blkChan     chan struct{}
	//emptyBlkChan		chan struct{}
	blkTxsLen   int
	recvBlkChan chan *blkCom.Block
	exitTx      chan struct{}
	exitBlk     chan struct{}
}

func NewSoloServer(chainId string) *SoloServer {
	SoloLog = logging.GetLogIns()
	solo := &SoloServer{
		chainId:     chainId,
		emptyEnable: false,
		blkTxsLen:   1000,
		//blkTimer:time.NewTicker(time.Second*1),
		blkChan: make(chan struct{}, 0),
		//emptyBlkChan: make(chan struct{}, 0),
		recvBlkChan: make(chan *blkCom.Block, 1000),
		exitTx:      make(chan struct{}),
		exitBlk:     make(chan struct{}),
	}
	return solo
}

func (solo *SoloServer) Start() {
	solo.blkNum, _ = ledger.GetBlockHeight()
	if solo.emptyEnable {
		solo.blkTimer = time.NewTicker(time.Second * 1)
	}
	if config.Config.EnableConsensus {
		go solo.monitor()
	}

	go solo.blockServer()
	p2pNetwork.RegisterConsNotify(solo.RecvBlk)
}

func (solo *SoloServer) Stop() {
	solo.exitTx <- struct{}{}
	solo.exitBlk <- struct{}{}
	SoloLog.Infof(LOGTABLE_SOLO, "stop solo")
}

func (solo *SoloServer) RecvBlk(msg []byte, msgSender uint64) error {
	if msg == nil {
		errStr := fmt.Sprintf("msg is nil")
		SoloLog.Warning(LOGTABLE_SOLO, errStr)
		return errors.New(errStr)
	}
	blk := blkCom.Block{}
	err := json.Unmarshal(msg, &blk)
	if err != nil {
		SoloLog.Warning(LOGTABLE_SOLO, err.Error())
		return err
	}
	solo.recvBlkChan <- &blk
	return nil
}

func (solo *SoloServer) blockServer() {
	for {
		select {
		case <-solo.blkChan:
			block := solo.makeBlock(false)
			if block == nil {
				continue
			}
			infoStr := fmt.Sprintf("new block %v", block)
			SoloLog.Info(LOGTABLE_SOLO, infoStr)
			solo.processBlk(block, true)
			infoStr = fmt.Sprintf("process new block ok")
			SoloLog.Info(LOGTABLE_SOLO, infoStr)
		//case <- solo.emptyBlkChan:
		//	block := solo.makeBlock(true)
		//	solo.processBlk(block, true)
		case block := <-solo.recvBlkChan:
			SoloLog.Infof(LOGTABLE_SOLO, "recv new block %v", block)
			solo.processBlk(block, false)
			SoloLog.Infof(LOGTABLE_SOLO, "process net new block %v ok", block)
		case <-solo.exitBlk:
			SoloLog.Infof(LOGTABLE_SOLO, "%s blk exit", solo.chainId)
			return
		}
	}
}

func (solo *SoloServer) monitor() {
	t := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-t.C:
			fmt.Println("tx timeout")
			txLen := txpool.IsTxsLenZero(solo.chainId)
			if txLen == false {
				solo.blkChan <- struct{}{}
			}
		//case <- solo.blkTimer.C:
		//	fmt.Println("empty timeout")
		//	solo.emptyBlkChan <- struct{}{}
		case <-solo.exitTx:
			SoloLog.Infof(LOGTABLE_SOLO, "%s tx exit", solo.chainId)
			return
		}
	}
}

func (solo *SoloServer) makeBlock(isEmpty bool) *blkCom.Block {
	if isEmpty {
		return &blkCom.Block{
			Header: &blkCom.Header{
				BlockNum: solo.blkNum,
			},
		}
	} else {
		txs, err := txpool.GetTxsFromTXPool(solo.chainId, solo.blkTxsLen)
		if err == nil {
			SoloLog.Infof(LOGTABLE_SOLO, "get txs count:%v", len(txs))
			return &blkCom.Block{
				Header: &blkCom.Header{
					BlockNum: solo.blkNum,
				},
				Txs: txs,
			}
		}
	}
	return nil
}

func serialize(blk *blkCom.Block) []byte {
	data, _ := json.Marshal(blk)
	return data
}

func (solo *SoloServer) writeBlock(blk *blkCom.Block) {
	fmt.Printf(" %s new block %d\n", solo.chainId, blk.Header.BlockNum)
	err := appMgr.BlockProcess(blk)
	if err != nil {
		SoloLog.Warning(LOGTABLE_SOLO, err.Error())
	}
	solo.blkNum++
	if len(blk.Txs) > 0 {
		txpool.DelTxs(solo.chainId, blk.Txs)
	}
}

func (solo *SoloServer) processBlk(blk *blkCom.Block, isSelf bool) {
	if isSelf {
		msg := serialize(blk)
		m := reqMsg.NewConsMsg(msg)
		p2pNetwork.Xmit(m, true)
	}
	solo.writeBlock(blk)

}
