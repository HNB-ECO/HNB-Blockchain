package common

import (
	"sync"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	"errors"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"fmt"
	"container/list"
)
var TXPoolLog logging.LogModule

const(
	LOGTABLE_TXPOOL string = "txpool"
)

type TXPool struct {
	sync.RWMutex
	txIndex		map[string]*list.Element
	txList		*list.List
}

func NewTXPool() *TXPool {
	TXPoolLog = logging.GetLogIns()
	return &TXPool{
		txList: list.New(),
		txIndex: make(map[string]*list.Element),
	}
}



func (tp *TXPool) GetTxListLen() int {
	tp.RLock()
	defer tp.RUnlock()
	return tp.txList.Len()
}

func (tp *TXPool) IsExists(txid string) (bool, error) {
	if txid == "" {
		errStr := fmt.Sprintf("txid is nil")
		return false, errors.New(errStr)
	}
	tp.RLock()
	defer tp.RUnlock()
	_, ok := tp.txIndex[txid]
	return ok, nil
}

func (tp *TXPool) AddTxToList(tx *common.Transaction) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	tp.Lock()
	defer tp.Unlock()
	if _, ok :=  tp.txIndex[tx.Txid]; ok {
		errStr := fmt.Sprintf("tx %s exists", tx.Txid)
		return errors.New(errStr)
	}
	ele := tp.txList.PushBack(tx)
	tp.txIndex[tx.Txid] = ele
	TXPoolLog.Infof(LOGTABLE_TXPOOL, "tx list len %d", len(tp.txIndex))
	return nil
}

func (tp *TXPool) DelTx(txid string) bool {
	tp.Lock()
	defer tp.Unlock()
	ele, ok :=  tp.txIndex[txid]
	if !ok || ele == nil {
		infoStr := fmt.Sprintf("tx %s not exists", txid)
		TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
		return false
	}
	tp.txList.Remove(ele)
	delete(tp.txIndex, txid)
	return true
}

func (tp *TXPool) DelTxs(txs []*common.Transaction) {
	tp.Lock()
	defer tp.Unlock()
	for _, tx := range txs {
		if tx == nil {
			infoStr := fmt.Sprintf("tx is nil")
			TXPoolLog.Warning(LOGTABLE_TXPOOL, infoStr)
			continue
		}
		ele, ok :=  tp.txIndex[tx.Txid]
		if !ok || ele == nil {
			infoStr := fmt.Sprintf("tx %s not exists", tx.Txid)
			TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
			continue
		}
		tp.txList.Remove(ele)
		delete(tp.txIndex, tx.Txid)
		infoStr := fmt.Sprintf("tx %s del succ", tx.Txid)
		TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
	}

}
func (tp *TXPool) DelTxsWithTxId(txids []string) {
	tp.Lock()
	defer tp.Unlock()
	for _, txid := range txids {
		ele, ok :=  tp.txIndex[txid]
		if !ok || ele == nil {
			infoStr := fmt.Sprintf("tx %s not exists", txid)
			TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
		}
		tp.txList.Remove(ele)
		delete(tp.txIndex, txid)
		infoStr := fmt.Sprintf("tx %s del succ", txid)
		TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
	}

}

func (tp *TXPool) GetTxsFromTXPool(count int) ([]*common.Transaction, error)  {
	if count <= 0 {
		errStr := fmt.Sprintf("tx count can not <= 0")
		return nil, errors.New(errStr)
	}
	targetCount := count
	txListLen := tp.GetTxListLen()
	if txListLen <= 0 {
		infoStr := fmt.Sprintf("no tx in txpool")
		TXPoolLog.Info(LOGTABLE_TXPOOL, infoStr)
		return nil, nil
	}
	if txListLen < targetCount {
		targetCount = txListLen
	}
	txs := make([]*common.Transaction, 0)
	txcount := 0
	for ele := tp.txList.Front(); (ele!= nil)&&(txcount<targetCount); ele = ele.Next() {
		tx, ok := ele.Value.(*common.Transaction)
		if !ok {
			errStr := fmt.Sprintf("tx parse err")
			return nil, errors.New(errStr)
		}
		txs = append(txs, tx)
		txcount++
	}
	return txs, nil
}

