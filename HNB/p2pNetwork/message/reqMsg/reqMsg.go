package reqMsg

import (
	msgCommon "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	mt "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/bean"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/server"
	"time"
)

func NewAddrs(nodeAddrs []msgCommon.PeerAddr) mt.Message {
	var addr mt.Addr
	addr.NodeAddrs = nodeAddrs

	return &addr
}

func NewAddrReq() mt.Message {
	var msg mt.AddrReq
	return &msg
}

func NewVerAck(isConsensus bool) mt.Message {
	var verAck mt.VerACK
	verAck.IsConsensus = isConsensus

	return &verAck
}

func NewVersion(n server.P2P, isCons bool, height uint32) mt.Message {
	var version mt.Version
	version.P = mt.VersionPayload{
		Version:      n.GetVersion(),
		Services:     n.GetServices(),
		SyncPort:     n.GetSyncPort(),
		ConsPort:     n.GetConsPort(),
		Nonce:        n.GetID(),
		IsConsensus:  isCons,
		HttpInfoPort: n.GetHttpInfoPort(),
		StartHeight:  uint64(height),
		TimeStamp:    time.Now().UnixNano(),
	}

	if n.GetRelay() {
		version.P.Relay = 1
	} else {
		version.P.Relay = 0
	}

	version.P.Cap[msgCommon.HTTP_INFO_FLAG] = 0x00

	return &version
}

func NewPingMsg(height uint64) *mt.Ping {
	var ping mt.Ping
	ping.Height = uint64(height)

	return &ping
}

func NewPongMsg(height uint64) *mt.Pong {
	var pong mt.Pong
	pong.Height = uint64(height)

	return &pong
}

func NewTestNet(msg string) *mt.TestNetwork {
	var pong mt.TestNetwork
	pong.Msg = msg

	return &pong
}

func NewTxMsg(msg []byte) *mt.TxMsg {
	var pong mt.TxMsg
	pong.Msg = msg

	return &pong
}

func NewSyncMsg(msg []byte) *mt.SyncMsg {
	var pong mt.SyncMsg
	pong.Msg = msg

	return &pong
}

func NewConsMsg(msg []byte) *mt.ConsMsg {
	var pong mt.ConsMsg
	pong.Msg = msg

	return &pong
}
