package txpool

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
)

type TXPoolInf interface {
	RecvTx(msg []byte)
	GetTxsFromTXPool(chainId string, count int) ([]*common.Transaction, error)
	TxsLen(chainId string) int
	DelTxs(chainId string, txs []*common.Transaction)
}

func RecvTx(msg []byte) error {
	return TxPoolIns.RecvTx(msg, 0)
}

func GetTxsFromTXPool(chainId string, count int) ([]*common.Transaction, error) {
	return TxPoolIns.GetTxsFromTXPool(chainId, count)
}

func DelTxs(chainId string, txs []*common.Transaction) {
	TxPoolIns.DelTxs(chainId, txs)
}

func IsTxsLenZero(chainId string) bool {
	return TxPoolIns.IsTxsLenZero(chainId)
}

func NotifyTx(chainId string) chan struct{} {
	return TxPoolIns.NotifyTx()
}

func GetPendingNonce(address common.Address) uint64 {
	return TxPoolIns.GetPendingNonce(address)
}

func GetContent() (map[common.Address]common.Transactions, map[common.Address]common.Transactions) {
	return TxPoolIns.GetContent()
}
