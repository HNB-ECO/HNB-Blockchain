package rest

import (
	"HNB/txpool"
	"HNB/util"
	"encoding/json"
	"bytes"
	"errors"
	"HNB/common"
)

type TxsInfo struct {
	Queue   []*TxInfo `json:"queue"`
	Pending []*TxInfo `json:"pending"`
}

type TxInfo struct {
	Nonce uint64 `json:"nonce"`
	From  string `json:"from"`
	Txid  string `json:"txid"`
}

func GetTxPoolQueue(params json.RawMessage)  (interface{}, error){
	queue, pending := txpool.GetContent()
	var queueInfo, pendingInfo []*TxInfo

	for k, v := range queue {
		for _, tx := range v {
			ti := &TxInfo{}
			ti.Txid = util.ByteToHex(tx.Txid.GetBytes())
			ti.From = util.ByteToHex(k.GetBytes())
			ti.Nonce = tx.Nonce()
			queueInfo = append(queueInfo, ti)
		}
	}

	for k, v := range pending {
		for _, tx := range v {
			ti := &TxInfo{}
			ti.Txid = util.ByteToHex(tx.Txid.GetBytes())
			ti.From = util.ByteToHex(k.GetBytes())
			ti.Nonce = tx.Nonce()
			pendingInfo = append(pendingInfo, ti)
		}
	}

	tsi := &TxsInfo{queueInfo, pendingInfo}
	return tsi, nil
}

func GetNonce(params json.RawMessage)  (interface{}, error){
	dec := json.NewDecoder(bytes.NewReader(params))
	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, errors.New("no [")
	}

	if !dec.More(){
		return nil, errors.New("data not complete")
	}

	var address string
	err := dec.Decode(&address)
	if err != nil{
		return nil, err
	}

	addr := common.Address{}
	addr.SetBytes(util.HexToByte(address))
	nonce := txpool.GetPendingNonce(addr)

	return nonce, nil
}