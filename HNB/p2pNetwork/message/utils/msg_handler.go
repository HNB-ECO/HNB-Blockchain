package utils

import (
	msgCommon "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	msgTypes "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/bean"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/server"
	"net"
	"strconv"
	"time"
)

// AddrReqHandle handles the neighbor address request from peer
func AddrReqHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {
	//log.Debug("receive addr request message", data.Addr, data.Id)
	remotePeer := p2p.GetPeer(data.Id)
	if remotePeer == nil {
		//log.Error("remotePeer invalid in AddrReqHandle")
		return
	}

	var addrStr []msgCommon.PeerAddr
	addrStr = p2p.GetNeighborAddrs()
	msg := reqMsg.NewAddrs(addrStr)
	err := p2p.Send(remotePeer, msg, false)
	if err != nil {
		//log.Error(err)
		return
	}
}

// VersionHandle handles version handshake protocol from peer
func VersionHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {
	//log.Debug("receive version message", data.Addr, data.Id)

	version := data.Payload.(*msgTypes.Version)

	remotePeer := p2p.GetPeerFromAddr(data.Addr)
	if remotePeer == nil {
		P2PLog.Warningf(LOGTABLE_NETWORK, "peer is not exist", data.Addr)
		//peer not exist,just remove list and return
		p2p.RemoveFromConnectingList(data.Addr)
		return
	}
	addrIp, err := msgCommon.ParseIPAddr(data.Addr)
	if err != nil {
		P2PLog.Warning(LOGTABLE_NETWORK, err.Error())
		return
	}
	nodeAddr := addrIp + ":" +
		strconv.Itoa(int(version.P.SyncPort))

	if version.P.IsConsensus == true {
		p := p2p.GetPeer(version.P.Nonce)

		if p == nil {
			P2PLog.Warningf(LOGTABLE_NETWORK, "sync link is not exist", version.P.Nonce)
			remotePeer.CloseCons()
			remotePeer.CloseSync()
			return
		} else {
			//p synclink must exist,merged
			p.ConsLink = remotePeer.ConsLink
			p.ConsLink.SetID(version.P.Nonce)
			p.SetConsState(remotePeer.GetConsState())
			remotePeer = p
		}
		if version.P.Nonce == p2p.GetID() {
			P2PLog.Warningf(LOGTABLE_NETWORK, "the node handshake with itself")
			p2p.SetOwnAddress(nodeAddr)
			p2p.RemoveFromInConnRecord(remotePeer.GetAddr())
			p2p.RemoveFromOutConnRecord(remotePeer.GetAddr())
			remotePeer.CloseCons()
			return
		}

		s := remotePeer.GetConsState()
		if s != msgCommon.INIT && s != msgCommon.HAND {
			P2PLog.Warningf(LOGTABLE_NETWORK, "unknown status to received version", s)
			remotePeer.CloseCons()
			return
		}

		// Todo: change the method of input parameters
		remotePeer.UpdateInfo(time.Now(), version.P.Version,
			version.P.Services, version.P.SyncPort,
			version.P.ConsPort, version.P.Nonce,
			version.P.Relay, version.P.StartHeight)

		var msg msgTypes.Message
		if s == msgCommon.INIT {
			remotePeer.SetConsState(msgCommon.HAND_SHAKE)
			//msg = reqMsg.NewVersion(p2p, true, ledger.DefLedger.GetCurrentBlockHeight())
			msg = reqMsg.NewVersion(p2p, true, 0)
		} else if s == msgCommon.HAND {
			remotePeer.SetConsState(msgCommon.HAND_SHAKED)
			msg = reqMsg.NewVerAck(true)

		}
		err := p2p.Send(remotePeer, msg, true)
		if err != nil {
			P2PLog.Error(LOGTABLE_NETWORK, err.Error())
			return
		}
	} else {
		if version.P.Nonce == p2p.GetID() {
			p2p.RemoveFromInConnRecord(remotePeer.GetAddr())
			p2p.RemoveFromOutConnRecord(remotePeer.GetAddr())

			P2PLog.Warningf(LOGTABLE_NETWORK, "the node handshake with itself")
			p2p.SetOwnAddress(nodeAddr)
			remotePeer.CloseSync()
			return
		}

		s := remotePeer.GetSyncState()
		if s != msgCommon.INIT && s != msgCommon.HAND {
			P2PLog.Warningf(LOGTABLE_NETWORK, "unknown status to received version %v", s)
			remotePeer.CloseSync()
			return
		}

		// Obsolete node
		p := p2p.GetPeer(version.P.Nonce)
		if p != nil {
			ipOld, err := msgCommon.ParseIPAddr(p.GetAddr())
			if err != nil {
				P2PLog.Warningf(LOGTABLE_NETWORK, "exist peer %d ip format is wrong %s", version.P.Nonce, p.GetAddr())
				return
			}
			ipNew, err := msgCommon.ParseIPAddr(data.Addr)
			if err != nil {
				remotePeer.CloseSync()
				P2PLog.Warningf(LOGTABLE_NETWORK, "connecting peer %d ip format is wrong %s, close", version.P.Nonce, data.Addr)
				return
			}
			if ipNew == ipOld {
				//same id and same ip
				n, ret := p2p.DelNbrNode(version.P.Nonce)
				if ret == true {
					P2PLog.Infof(LOGTABLE_NETWORK, "peer reconnect %d", version.P.Nonce)
					// Close the connection and release the node source
					n.CloseSync()
					n.CloseCons()
					//TODO jm
				}
			} else {
				P2PLog.Infof(LOGTABLE_NETWORK, "same peer id from different addr: %s, %s close latest one", ipOld, ipNew)
				remotePeer.CloseSync()
				return

			}
		}

		if version.P.Cap[msgCommon.HTTP_INFO_FLAG] == 0x01 {
			remotePeer.SetHttpInfoState(true)
		} else {
			remotePeer.SetHttpInfoState(false)
		}
		remotePeer.SetHttpInfoPort(version.P.HttpInfoPort)

		remotePeer.UpdateInfo(time.Now(), version.P.Version,
			version.P.Services, version.P.SyncPort,
			version.P.ConsPort, version.P.Nonce,
			version.P.Relay, version.P.StartHeight)
		remotePeer.SyncLink.SetID(version.P.Nonce)
		p2p.AddNbrNode(remotePeer)

		//TODO jm
		//if pid != nil {
		//	input := &msgCommon.AppendPeerID{
		//		ID: version.P.Nonce,
		//	}
		//	pid.Tell(input)
		//}

		var msg msgTypes.Message
		if s == msgCommon.INIT {
			remotePeer.SetSyncState(msgCommon.HAND_SHAKE)
			msg = reqMsg.NewVersion(p2p, false, 0)
		} else if s == msgCommon.HAND {
			remotePeer.SetSyncState(msgCommon.HAND_SHAKED)
			msg = reqMsg.NewVerAck(false)
		}
		err := p2p.Send(remotePeer, msg, false)
		if err != nil {
			P2PLog.Error(LOGTABLE_NETWORK, err.Error())
			return
		}
	}
}

// VerAckHandle handles the version ack from peer
func VerAckHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {
	//log.Debug("receive verAck message from ", data.Addr, data.Id)

	verAck := data.Payload.(*msgTypes.VerACK)
	remotePeer := p2p.GetPeer(data.Id)

	if remotePeer == nil {
		P2PLog.Warningf(LOGTABLE_NETWORK, "nbr node is not exist", data.Id, data.Addr)
		return
	}

	if verAck.IsConsensus == true {
		s := remotePeer.GetConsState()
		if s != msgCommon.HAND_SHAKE && s != msgCommon.HAND_SHAKED {
			P2PLog.Warningf(LOGTABLE_NETWORK, "unknown status to received verAck", s)
			return
		}

		remotePeer.SetConsState(msgCommon.ESTABLISH)
		p2p.RemoveFromConnectingList(data.Addr)
		remotePeer.SetConsConn(remotePeer.GetConsConn())

		if s == msgCommon.HAND_SHAKE {
			msg := reqMsg.NewVerAck(true)
			p2p.Send(remotePeer, msg, true)
		}
	} else {
		s := remotePeer.GetSyncState()
		if s != msgCommon.HAND_SHAKE && s != msgCommon.HAND_SHAKED {
			P2PLog.Warningf(LOGTABLE_NETWORK, "unknown status to received verAck", s)
			return
		}

		remotePeer.SetSyncState(msgCommon.ESTABLISH)
		p2p.RemoveFromConnectingList(data.Addr)
		remotePeer.DumpInfo()

		addr := remotePeer.SyncLink.GetAddr()

		if s == msgCommon.HAND_SHAKE {
			msg := reqMsg.NewVerAck(false)
			p2p.Send(remotePeer, msg, false)
		} else {
			//consensus port connect
			if remotePeer.GetConsPort() > 0 {
				addrIp, err := msgCommon.ParseIPAddr(addr)
				if err != nil {
					P2PLog.Warning(LOGTABLE_NETWORK, err.Error())
					return
				}
				nodeConsensusAddr := addrIp + ":" +
					strconv.Itoa(int(remotePeer.GetConsPort()))
				go p2p.Connect(nodeConsensusAddr, true)
			}
		}

		msg := reqMsg.NewAddrReq()
		go p2p.Send(remotePeer, msg, false)
	}

}

// AddrHandle handles the neighbor address response message from peer
func AddrHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {
	//log.Debug("handle addr message", data.Addr, data.Id)

	var msg = data.Payload.(*msgTypes.Addr)
	P2PLog.Debugf(LOGTABLE_NETWORK, "get addr %v", msg.NodeAddrs)
	for _, v := range msg.NodeAddrs {
		var ip net.IP
		ip = v.IpAddr[:]
		address := ip.To16().String() + ":" + strconv.Itoa(int(v.SyncPort))

		if v.ID == p2p.GetID() {
			continue
		}

		if p2p.NodeEstablished(v.ID) {
			continue
		}

		if ret := p2p.GetPeerFromAddr(address); ret != nil {
			continue
		}

		if v.SyncPort == 0 {
			continue
		}
		if p2p.IsAddrFromConnecting(address) {
			continue
		}
		//log.Info("connect ip address:", address)
		P2PLog.Debugf(LOGTABLE_NETWORK, "try connnect %v", address)
		go func() {
			err := p2p.Connect(address, false)
			if err != nil {
				P2PLog.Debugf(LOGTABLE_NETWORK, "try connnect %v fail %v",
					address, err.Error())
			}
		}()
	}
}

// DisconnectHandle handles the disconnect events
func DisconnectHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {
	p2p.RemoveFromInConnRecord(data.Addr)
	p2p.RemoveFromOutConnRecord(data.Addr)
	remotePeer := p2p.GetPeer(data.Id)
	if remotePeer == nil {
		return
	}
	p2p.RemoveFromConnectingList(data.Addr)

	if remotePeer.SyncLink.GetAddr() == data.Addr {
		p2p.RemovePeerSyncAddress(data.Addr)
		p2p.RemovePeerConsAddress(data.Addr)
		remotePeer.CloseSync()
		remotePeer.CloseCons()
	}
	if remotePeer.ConsLink.GetAddr() == data.Addr {
		p2p.RemovePeerConsAddress(data.Addr)
		remotePeer.CloseCons()
	}
}

//PingHandle handle ping msg from peer
func PingHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {

	ping := data.Payload.(*msgTypes.Ping)
	remotePeer := p2p.GetPeer(data.Id)
	if remotePeer == nil {
		//		log.Error("remotePeer invalid in PingHandle")
		return
	}
	remotePeer.SetHeight(ping.Height)

	height := 0
	p2p.SetHeight(uint64(height))
	msg := reqMsg.NewPongMsg(uint64(height))

	err := p2p.Send(remotePeer, msg, false)
	if err != nil {
		//		log.Error(err)
	}
}

func PongHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {

	pong := data.Payload.(*msgTypes.Pong)

	remotePeer := p2p.GetPeer(data.Id)
	if remotePeer == nil {
		//		log.Error("remotePeer invalid in PongHandle")
		return
	}
	remotePeer.SetHeight(pong.Height)
}

func NetHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {

	msg := data.Payload.(*msgTypes.TestNetwork)

	P2PLog.Info(LOGTABLE_NETWORK, msg.Msg)
}

func (mr *MessageRouter) TxHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {

	msg := data.Payload.(*msgTypes.TxMsg)
	if mr.txFunc != nil {
		mr.txFunc(msg.Msg, data.Id)
	}
}

func (mr *MessageRouter) SyncHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {
	msg := data.Payload.(*msgTypes.SyncMsg)
	if mr.syncFunc != nil {
		mr.syncFunc(msg.Msg, data.Id)
	}
}

func (mr *MessageRouter) ConsHandle(data *msgTypes.MsgPayload, p2p server.P2P, args ...interface{}) {

	msg := data.Payload.(*msgTypes.ConsMsg)
	if mr.consFunc != nil {
		mr.consFunc(msg.Msg, data.Id)
	}
}
