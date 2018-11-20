package txpool

import (
	"HNB/common"
)

type TXPoolInf interface {
	RecvTx(msg []byte)
	GetTxsFromTXPool(chainId string, count int) ([]*common.Transaction, error)
	TxsLen(chainId string) int
	DelTxs(chainId string, txs []*common.Transaction)
}

func RecvTx(msg []byte) {
	TxPoolIns.RecvTx(msg, 0)
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
