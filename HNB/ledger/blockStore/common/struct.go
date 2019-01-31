package common

import (
	txComm "HNB/common"
	"HNB/util"
)

type Header struct {
	BlockNum     uint64        `json:"blockNum"`
	PreviousHash util.HexBytes `json:"previousHash"`
	CreateTime   uint64        `json:"createTime"`
	SenderId     util.HexBytes `json:"senderId"`
	ConsArgs     util.HexBytes `json:"consArgs"`
	Ext          util.HexBytes `json:"ext"`
	TxsHash      util.HexBytes `json:"txsHash"`
	StateHash    util.HexBytes `json:"stateHash"`
}

type Block struct {
	Header *Header               `json:"header"`
	Txs    []*txComm.Transaction `json:"txs"`
}
