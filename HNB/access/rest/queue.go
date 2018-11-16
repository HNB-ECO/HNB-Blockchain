package rest

import (
	"github.com/gocraft/web"
	"net/http"
	"encoding/json"
	"HNB/txpool"
	"HNB/util"
)


type TxsInfo struct {
	Queue []*TxInfo    `json:"queue"`
	Pending []*TxInfo  `json:"pending"`
}


type TxInfo struct {
	Nonce uint64 `json:"nonce"`
	From string	 `json:"from"`
	Txid string  `json:"txid"`
}

func (*serverREST) GetTxPoolQueue(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	queue, pending := txpool.GetContent()
	var queueInfo,pendingInfo []*TxInfo


	for k,v := range queue{
		for _, tx := range v{
			ti := &TxInfo{}
			ti.Txid = util.ByteToHex(tx.Txid.GetBytes())
			ti.From = util.ByteToHex(k.GetBytes())
			ti.Nonce = tx.Nonce()
			queueInfo = append(queueInfo, ti)
		}
	}

	for k,v := range pending{
		for _, tx := range v{
			ti := &TxInfo{}
			ti.Txid = util.ByteToHex(tx.Txid.GetBytes())
			ti.From = util.ByteToHex(k.GetBytes())
			ti.Nonce = tx.Nonce()
			pendingInfo = append(pendingInfo, ti)
		}
	}


	tsi := &TxsInfo{queueInfo, pendingInfo}
	retMsg := FormatQueryResResult("0000", "", tsi)
	encoder.Encode(retMsg)
}


