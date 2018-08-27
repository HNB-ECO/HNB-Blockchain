package worker

import (
	tpComm "github.com/HNB-ECO/HNB-Blockchain/HNB/txpool/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	"fmt"
	"encoding/json"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
)

var TXPoolLog logging.LogModule

const(
	LOGTABLE_TXPOOL string = "txpool"
)


type TXPoolWorker struct {
	txPool			*tpComm.TXPool
	recvTxChan		chan *common.Transaction
	exit			chan struct{}
}

func NewTXPoolWorker() *TXPoolWorker {
	TXPoolLog = logging.GetLogIns()
	tpw := &TXPoolWorker{
		txPool:tpComm.NewTXPool(),
		recvTxChan: make(chan *common.Transaction, 1000),
		exit: make(chan struct{}),
	}
	go tpw.handlerTx()
	return tpw
}

func (tpw *TXPoolWorker) RecvTx(tx *common.Transaction)  {
	select {
	case tpw.recvTxChan <- tx:
	default:
		errStr := fmt.Sprintf("tx chan full")
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
	}

}

func (tpw *TXPoolWorker) CheckTx(tx *common.Transaction) bool {
	if tx == nil {
		errStr := fmt.Sprintf("tx is nil")
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
		return false
	}
	ok ,err := tpw.txPool.IsExists(tx.Txid)
	if err != nil  {
		TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
		return false
	}
	if ok {
		errStr := fmt.Sprintf("tx Exists %s", tx.Txid)
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
		return false
	}
	return tpw.CheckSign()
}
func (tpw *TXPoolWorker) CheckSign() bool {
	return true
}

func (tpw *TXPoolWorker) handlerTx() {
	for  {
		select {
		case tx := <-tpw.recvTxChan:
			infoStr := fmt.Sprintf("check tx %s", tx.Txid)
			TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
			if tpw.CheckTx(tx) {
				data, err := json.Marshal(tx)
				if err != nil {
					TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
					continue
				}
				infoStr := fmt.Sprintf("check tx ok %s", tx.Txid)
				TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
				m := reqMsg.NewTxMsg(data)
				p2pNetwork.Xmit(m, false)
				infoStr = fmt.Sprintf("broad tx %s", tx.Txid)
				TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
				err = tpw.txPool.AddTxToList(tx)
				if err != nil {
					errStr := fmt.Sprintf("add tx err %s", err.Error())
					TXPoolLog.Info(LOGTABLE_TXPOOL, errStr)
				}
				continue
			}
			TXPoolLog.Infof(LOGTABLE_TXPOOL, "check tx %s failed", tx.Txid)
		case <-tpw.exit:
			TXPoolLog.Infof(LOGTABLE_TXPOOL, "exit")
			return
		}
	}
}

func (tpw *TXPoolWorker) GetTxsFromTXPool(count int) ([]*common.Transaction, error) {
	return tpw.txPool.GetTxsFromTXPool(count)
}

func (tpw *TXPoolWorker) TxsLen() int {
	return tpw.txPool.GetTxListLen()
}


func (tpw *TXPoolWorker) DelTxs (txs []*common.Transaction) {
	tpw.txPool.DelTxs(txs)
}
