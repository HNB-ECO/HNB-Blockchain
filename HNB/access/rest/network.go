package rest

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"encoding/json"
)

//查询节点链接信息
func GetAddr(params json.RawMessage) (interface{}, error) {
	peers := p2pNetwork.GetNeighborAddrs()
	return peers, nil
}
