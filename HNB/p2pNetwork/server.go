package p2pNetwork

import (
	"HNB/p2pNetwork/common"
	"HNB/p2pNetwork/message/bean"
	"HNB/p2pNetwork/peer"
)

//p2pNetwork提供对外服务, 根据需求后续添加
func GetNp() *peer.NbrPeers {
	return P2PIns.network.GetNp()
}

func Xmit(msg bean.Message, isCons bool) {
	P2PIns.network.Xmit(msg, isCons)
}

func GetNeighborAddrs() []common.PeerAddr {
	return P2PIns.network.GetNeighborAddrs()
}

func GetNeighbors() []*peer.Peer {
	return P2PIns.network.GetNeighbors()
}

func Send(peerid uint64, msg bean.Message, isConsensus bool) error {
	peer := P2PIns.network.GetPeer(peerid)
	return P2PIns.network.Send(peer, msg, isConsensus)
}

func GetLocatePeerID() uint64 {
	return P2PIns.GetID()
}

//func Send(p *peer.Peer, msg bean.Message, isConsensus bool) error{
//	return P2PIns.network.Send(p, msg, isConsensus)
//}

func RegisterTxNotify(nf common.NotifyFunc) {
	P2PIns.msgRouter.RegisterTxNotify(nf)
}

func RegisterSyncNotify(nf common.NotifyFunc) {
	P2PIns.msgRouter.RegisterSyncNotify(nf)
}

func RegisterConsNotify(nf common.NotifyFunc) {
	P2PIns.msgRouter.RegisterConsNotify(nf)
}
