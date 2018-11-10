package rest

import (
	"HNB/p2pNetwork"
	"encoding/json"
	"github.com/gocraft/web"
	"net/http"
)

//查询节点链接信息
func (*serverREST) GetAddr(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	peers := p2pNetwork.GetNeighborAddrs()
	retMsg := FormatQueryResResult("0000", "", peers)
	encoder.Encode(retMsg)
}
