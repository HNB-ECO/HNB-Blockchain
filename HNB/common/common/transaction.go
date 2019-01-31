package common

import (
	"math/big"
)

const (
	HGS = "hgs"
	HNB = "hnb"
)

type Address [20]byte
type Hash [32]byte
type Transactions []*Transaction

func (h *Hash) GetBytes() []byte {
	var m []byte
	m = make([]byte, 32)
	copy(m, h[:])
	return m
}

func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-20:]
	}
	copy(a[20-len(b):], b)
}

func (a *Address) GetBytes() []byte {
	var m []byte
	m = make([]byte, 20)
	copy(m, a[:])
	return m
}

type Transaction struct {
	ContractName string  `json:"contractName"` //交易类型
	Payload      []byte  `json:"payload"`      //具体交易的数据序列化后的
	Txid         Hash    `json:"txid"`         //交易的
	From         Address `json:"from"`         //账户发送方
	NonceValue   uint64  `json:"nonceValue"`

	V *big.Int `json:"v"` //`json:"v" gencodec:"required"`
	R *big.Int `json:"r"` //`json:"r" gencodec:"required"`
	S *big.Int `json:"s"` //`json:"s" gencodec:"required"`

}

func NewTransaction() *Transaction {
	return new(Transaction)
}

func (tx *Transaction) Hash() Hash {
	return tx.Txid
}

func (tx *Transaction) Nonce() uint64 {
	return tx.NonceValue
}

func (tx *Transaction) FromAddress() Address {
	return tx.From
}

type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].NonceValue < s[j].NonceValue }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
