package rest

import (
	"github.com/gocraft/web"
	"net/http"
	"encoding/json"
	"HNB/txpool"
	"HNB/common"
)


type txInfo struct {
	Queue map[common.Address]common.Transactions
	Pending map[common.Address]common.Transactions
}

func (*serverREST) GetTxPoolQueue(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	queue, pending := txpool.GetContent()
	ti := &txInfo{queue, pending}
	retMsg := FormatQueryResResult("0000", "", ti)
	encoder.Encode(retMsg)
}


