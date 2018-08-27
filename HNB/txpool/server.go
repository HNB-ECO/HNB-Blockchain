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

func RecvTx(msg []byte) {
	TxPool.RecvTx(msg)
}

func GetTxsFromTXPool(chainId string, count int) ([]*common.Transaction, error)  {
	return TxPool.GetTxsFromTXPool(chainId, count)
}

func TxsLen(chainId string) int  {
	return TxPool.TxsLen(chainId)
}

func DelTxs(chainId string, txs []*common.Transaction)  {
	TxPool.DelTxs(chainId, txs)
}
