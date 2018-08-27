package txpool

import (
	"sync"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"fmt"
	"errors"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	tpw "github.com/HNB-ECO/HNB-Blockchain/HNB/txpool/worker"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"encoding/json"
)
var TXPoolLog logging.LogModule

var TxPool *TXPoolServer
const(
	LOGTABLE_TXPOOL string = "txpool"
	HGS             string = "hgs"
)

type TXPoolServer struct {
	sync.RWMutex
	worker			map[string]*tpw.TXPoolWorker
}

func NewTXPoolServer() *TXPoolServer {
	TXPoolLog = logging.GetLogIns()
	s := &TXPoolServer{
		worker:make(map[string]*tpw.TXPoolWorker),
	}
	TxPool = s
	return s
}
func (tps *TXPoolServer) IsExists(chainId string) bool {
	tps.Lock()
	defer tps.Unlock()
	_, ok := tps.worker[chainId]
	return ok
}

func (tps *TXPoolServer) RecvTx(msg []byte) {
	if msg == nil {
		errStr := fmt.Sprintf("msg is nil")
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
		return
	}
	tx := common.Transaction{}
	err := json.Unmarshal(msg, &tx)
	if err != nil {
		TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
		return
	}
	worker, err := tps.GetServer(tx.Type)
	if err != nil {
		TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
		return
	}
	infoStr := fmt.Sprintf("recv tx  %v", tx)
	TXPoolLog.Debugf(LOGTABLE_TXPOOL, infoStr)
	worker.RecvTx(&tx)
}

func (tps * TXPoolServer) TxsLen(chainId string) int {
	worker, err := tps.GetServer(chainId)
	if err != nil {
		TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
		return 0
	}
	return worker.TxsLen()
}

func (tps *TXPoolServer) SetServer(chainId string, tpw *tpw.TXPoolWorker)  {
	if chainId == "" || tpw == nil {
		errStr := fmt.Sprintf("chainId[%s], tpw is nil[%t]", chainId, tpw == nil)
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
		return
	}
	tps.Lock()
	defer tps.Unlock()
	if _, ok := tps.worker[chainId]; ok {
		errStr := fmt.Sprintf("chainId exists")
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
		return
	}
	tps.worker[chainId] = tpw
	TXPoolLog.Infof(LOGTABLE_TXPOOL, "worker %v", tps.worker)
}

func (tps *TXPoolServer) GetServer(chainId string) (*tpw.TXPoolWorker, error) {
	if chainId == "" {
		errStr := fmt.Sprintf("chainId[%s] is nil", chainId)
		return nil, errors.New(errStr)
	}
	tps.RLock()
	defer tps.RUnlock()
	tpw, ok := tps.worker[chainId]
	if !ok {
		errStr := fmt.Sprintf("chainId not exists")
		return nil, errors.New(errStr)
	}
	return tpw, nil
}

func (tps *TXPoolServer) InitTXPoolServer() {
	worker := tpw.NewTXPoolWorker()
	tps.SetServer(HGS, worker)
	p2pNetwork.RegisterTxNotify(tps.RecvTx)

}


func (tps *TXPoolServer) GetTxsFromTXPool(chainId string, count int) ([]*common.Transaction, error) {
	worker, err := tps.GetServer(chainId)
	if err != nil {
		TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
		return nil, errors.New(err.Error())
	}
	return worker.GetTxsFromTXPool(count)
}

func (tps *TXPoolServer) DelTxs(chainId string, txs []*common.Transaction) {
	worker, err := tps.GetServer(chainId)
	if err != nil {
		TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
		return
	}
	worker.DelTxs(txs)
}



