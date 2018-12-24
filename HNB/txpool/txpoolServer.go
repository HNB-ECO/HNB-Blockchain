package txpool

import (
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/pkg/errors"
	//"HNB/p2pNetwork/message/reqMsg"
	//"encoding/json"
)

var TXPoolLog logging.LogModule

var TxPoolIns *TXPoolServer

const (
	LOGTABLE_TXPOOL string = "txpool"
	HGS             string = "hgs"
	HNB             string = "hnb"
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

func (tps *TXPoolServer) RecvTx(msg []byte, msgSender uint64) error {
	if msg == nil {
		errStr := fmt.Sprintf("msg is nil")
		TXPoolLog.Warning(LOGTABLE_TXPOOL, errStr)
		return errors.New(errStr)
	}

	tx := common.Transaction{}
	err := json.Unmarshal(msg, &tx)
	if err != nil {
		TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
		return err
	}

	if msgSender == 0 { //local
		m := reqMsg.NewTxMsg(msg)
		p2pNetwork.Xmit(m, false)
		tps.txpool.AddLocal(&tx)
	} else {
		tps.txpool.AddRemote(&tx)
	}

	infoStr := fmt.Sprintf("recv tx  %v", string(msg))
	TXPoolLog.Debugf(LOGTABLE_TXPOOL, infoStr)

	//msgTx := &RecvTxStruct{}
	//if msgSender == 0{
	//	msgTx.local = true
	//}else{
	//	msgTx.local = false
	//}
	//TXPoolLog.Debugf(LOGTABLE_TXPOOL, "recv msg local:%v", msgTx.local)
	//
	//msgTx.recvTx = msg
	//tps.txpool.recvTx <- msgTx
	return nil
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
