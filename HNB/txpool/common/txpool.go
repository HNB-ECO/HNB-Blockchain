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


