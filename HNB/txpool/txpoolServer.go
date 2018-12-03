package txpool

import (
	"HNB/common"
	"HNB/logging"
	"HNB/p2pNetwork"
	"fmt"
)

var TXPoolLog logging.LogModule

var TxPoolIns *TXPoolServer

const (
	LOGTABLE_TXPOOL string = "txpool"
	HGS             string = "hgs"
)

type TXPoolServer struct {
	txpool *TxPool
}

func NewTXPoolServer() *TXPoolServer {
	TXPoolLog = logging.GetLogIns()

	s := &TXPoolServer{
		NewTxPool(DefaultTxPoolConfig),
	}
	TxPoolIns = s
	return s
}

func (tps *TXPoolServer) RecvTx(msg []byte, msgSender uint64) {
	if msg == nil {
		errStr := fmt.Sprintf("msg is nil")
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
		return
	}

	msgTx := &RecvTxStruct{}
	if msgSender == 0{
		msgTx.local = true
	}else{
		msgTx.local = false
	}
	TXPoolLog.Debugf(LOGTABLE_TXPOOL, "recv msg local:%v", msgTx.local)

	msgTx.recvTx = msg
	tps.txpool.recvTx <- msgTx
}

func (tps *TXPoolServer) GetTxsFromTXPool(chainId string, count int) ([]*common.Transaction, error) {
	txs, err := tps.txpool.Pending()
	if err != nil {
		return nil, err
	}

	var dstTxs []*common.Transaction
	var dstCount int = 0

	for _, addrTxs := range txs {
		for _, v := range addrTxs {
			dstTxs = append(dstTxs, v)
			dstCount++
			if dstCount == count {
				return dstTxs, nil
			}
		}
	}

	return dstTxs, nil
}

func (tps *TXPoolServer) Start() {
	p2pNetwork.RegisterTxNotify(tps.RecvTx)
}

func (tps *TXPoolServer) DelTxs(chainId string, txs []*common.Transaction) {
	tps.txpool.recvBlkTxs <- txs
}

func (tps *TXPoolServer) IsTxsLenZero(chainId string) bool {
	pending, _ := tps.txpool.Pending()
	return len(pending) == 0
}

func (tps *TXPoolServer) NotifyTx() chan struct{} {
	return tps.txpool.GetNotify()
}

func (tps *TXPoolServer) GetPendingNonce(address common.Address) uint64 {
	return tps.txpool.State().GetNonce(address)
}


func (tps *TXPoolServer) GetContent() (map[common.Address]common.Transactions, map[common.Address]common.Transactions) {
	return tps.txpool.Content()
}

