package p2pNetwork

import (
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/bean"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/peer"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/server"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

func NewNetServer() server.P2P {
	n := &NetSubServer{
		SyncChan: make(chan *bean.MsgPayload, common.CHAN_CAPABILITY),
		ConsChan: make(chan *bean.MsgPayload, common.CHAN_CAPABILITY),
	}

	n.PeerAddrMap.PeerSyncAddress = make(map[string]*peer.Peer)
	n.PeerAddrMap.PeerConsAddress = make(map[string]*peer.Peer)

	n.init()
	return n
}

type NetSubServer struct {
	peerConn     peer.PeerCom
	synclistener net.Listener
	conslistener net.Listener
	SyncChan     chan *bean.MsgPayload
	ConsChan     chan *bean.MsgPayload
	ConnectingNodes
	PeerAddrMap
	Np            *peer.NbrPeers
	connectLock   sync.Mutex
	inConnRecord  InConnectionRecord
	outConnRecord OutConnectionRecord
	OwnAddress    string
}

type InConnectionRecord struct {
	sync.RWMutex
	InConnectingAddrs []string
}

type OutConnectionRecord struct {
	sync.RWMutex
	OutConnectingAddrs []string
}

type ConnectingNodes struct {
	sync.RWMutex
	ConnectingAddrs []string
}

type PeerAddrMap struct {
	sync.RWMutex
	PeerSyncAddress map[string]*peer.Peer
	PeerConsAddress map[string]*peer.Peer
}

func (ns *NetSubServer) init() error {
	ns.peerConn.SetVersion(common.PROTOCOL_VERSION)

	if config.Config.EnableConsensus {
		ns.peerConn.SetServices(uint64(common.VERIFY_NODE))
	} else {
		ns.peerConn.SetServices(uint64(common.SERVICE_NODE))
	}

	ns.peerConn.SetSyncPort(config.Config.SyncPort)
	ns.peerConn.SetConsPort(config.Config.ConsPort)

	ns.peerConn.SetRelay(true)

	rand.Seed(time.Now().UnixNano())
	id := rand.Uint64()

	ns.peerConn.SetID(id)

	ns.Np = &peer.NbrPeers{}
	ns.Np.Init()

	logmsg := fmt.Sprintf("peerID:%v, sync port:%v, cons port:%v",
		id, config.Config.SyncPort, config.Config.ConsPort)
	P2PLog.Info(LOGTABLE_NETWORK, logmsg)

	return nil
}

func (ns *NetSubServer) Start() {
	ns.startListening()
}

func (ns *NetSubServer) GetVersion() uint32 {
	return ns.peerConn.GetVersion()
}

func (ns *NetSubServer) GetID() uint64 {
	return ns.peerConn.GetID()
}

func (ns *NetSubServer) SetHeight(height uint64) {
	ns.peerConn.SetHeight(height)
}

func (ns *NetSubServer) GetHeight() uint64 {
	return ns.peerConn.GetHeight()
}

func (ns *NetSubServer) GetTime() int64 {
	t := time.Now()
	return t.UnixNano()
}

func (ns *NetSubServer) GetServices() uint64 {
	return ns.peerConn.GetServices()
}

func (ns *NetSubServer) GetSyncPort() uint16 {
	return ns.peerConn.GetSyncPort()
}

func (ns *NetSubServer) GetConsPort() uint16 {
	return ns.peerConn.GetConsPort()
}

func (ns *NetSubServer) GetHttpInfoPort() uint16 {
	return ns.peerConn.GetHttpInfoPort()
}

func (ns *NetSubServer) GetRelay() bool {
	return ns.peerConn.GetRelay()
}

func (ns *NetSubServer) GetPeer(id uint64) *peer.Peer {
	return ns.Np.GetPeer(id)
}

func (ns *NetSubServer) GetNp() *peer.NbrPeers {
	return ns.Np
}

func (ns *NetSubServer) GetNeighborAddrs() []common.PeerAddr {
	return ns.Np.GetNeighborAddrs()
}

func (ns *NetSubServer) GetConnectionCnt() uint32 {
	return ns.Np.GetNbrNodeCnt()
}

func (ns *NetSubServer) AddNbrNode(remotePeer *peer.Peer) {
	ns.Np.AddNbrNode(remotePeer)
}

func (ns *NetSubServer) DelNbrNode(id uint64) (*peer.Peer, bool) {
	return ns.Np.DelNbrNode(id)
}

func (ns *NetSubServer) GetNeighbors() []*peer.Peer {
	return ns.Np.GetNeighbors()
}

func (ns *NetSubServer) NodeEstablished(id uint64) bool {
	return ns.Np.NodeEstablished(id)
}

func (ns *NetSubServer) Xmit(msg bean.Message, isCons bool) {
	ns.Np.Broadcast(msg, isCons)
}

func (ns *NetSubServer) GetMsgChan(isConsensus bool) chan *bean.MsgPayload {
	if isConsensus {
		return ns.ConsChan
	} else {
		return ns.SyncChan
	}
}

func (ns *NetSubServer) Send(p *peer.Peer, msg bean.Message, isConsensus bool) error {
	if p != nil {
		return p.Send(msg, isConsensus)
	}
	return errors.New("send to a invalid peer")
}

func (ns *NetSubServer) IsPeerEstablished(p *peer.Peer) bool {
	if p != nil {
		return ns.Np.NodeEstablished(p.GetID())
	}
	return false
}

func (ns *NetSubServer) Connect(addr string, isConsensus bool) error {
	if ns.IsAddrInOutConnRecord(addr) {
		P2PLog.Warningf(LOGTABLE_NETWORK, "addr:%s is out conn", addr)
		return nil
	}
	if ns.IsOwnAddress(addr) {
		return nil
	}
	if !ns.AddrValid(addr) {
		return nil
	}

	ns.connectLock.Lock()
	connCount := uint(ns.GetOutConnRecordLen())
	if connCount >= config.Config.MaxConnOutBound {
		P2PLog.Warningf(LOGTABLE_NETWORK, "Connect: out connections(%d) reach the max limit(%d)",
			connCount, config.Config.MaxConnOutBound)
		ns.connectLock.Unlock()
		return errors.New("connect: out connections reach the max limit")
	}
	ns.connectLock.Unlock()

	if ns.IsNbrPeerAddr(addr, isConsensus) {
		return nil
	}

	ns.connectLock.Lock()
	if added := ns.AddOutConnectingList(addr); added == false {
		p := ns.GetPeerFromAddr(addr)
		if p != nil {
			if p.SyncLink.Valid() {
				//log.Info("node exist in connecting list", addr)
				ns.connectLock.Unlock()
				return errors.New("node exist in connecting list")
			}
		}
		ns.RemoveFromConnectingList(addr)
	}
	ns.connectLock.Unlock()

	isTls := config.Config.IsPeersTLS
	var conn net.Conn
	var err error
	var remotePeer *peer.Peer
	if isTls {
		conn, err = TLSDial(addr)
		if err != nil {
			ns.RemoveFromConnectingList(addr)
			//log.Error("connect failed: ", err)
			return err
		}
	} else {
		conn, err = nonTLSDial(addr)
		if err != nil {
			ns.RemoveFromConnectingList(addr)
			//log.Error("connect failed: ", err)
			return err
		}
	}

	addr = conn.RemoteAddr().String()

	msg := fmt.Sprintf("isConsensus:%v peer %s connect with %s with %s",
		isConsensus, conn.LocalAddr().String(), conn.RemoteAddr().String(),
		conn.RemoteAddr().Network())
	P2PLog.Info(LOGTABLE_NETWORK, msg)

	if !isConsensus {
		ns.AddOutConnRecord(addr)
		remotePeer = peer.NewPeer()
		ns.AddPeerSyncAddress(addr, remotePeer)
		remotePeer.SyncLink.SetAddr(addr)
		remotePeer.SyncLink.SetConn(conn)
		remotePeer.AttachSyncChan(ns.SyncChan)
		go remotePeer.SyncLink.Rx()
		remotePeer.SetSyncState(common.HAND)
	} else {
		remotePeer = peer.NewPeer()
		ns.AddPeerConsAddress(addr, remotePeer)
		remotePeer.ConsLink.SetAddr(addr)
		remotePeer.ConsLink.SetConn(conn)
		remotePeer.AttachConsChan(ns.ConsChan)
		go remotePeer.ConsLink.Rx()
		remotePeer.SetConsState(common.HAND)
	}
	version := reqMsg.NewVersion(ns, isConsensus, 0)
	err = remotePeer.Send(version, isConsensus)
	if err != nil {
		if !isConsensus {
			ns.RemoveFromOutConnRecord(addr)
		}
		P2PLog.Error(LOGTABLE_NETWORK, err.Error())
		return err
	}
	return nil
}

func (ns *NetSubServer) Halt() {
	peers := ns.Np.GetNeighbors()
	for _, p := range peers {
		p.CloseSync()
		p.CloseCons()
	}
	if ns.synclistener != nil {
		ns.synclistener.Close()
	}
	if ns.conslistener != nil {
		ns.conslistener.Close()
	}
}

func (ns *NetSubServer) startListening() error {
	var err error

	syncPort := ns.peerConn.GetSyncPort()
	consPort := ns.peerConn.GetConsPort()

	err = ns.startSyncListening(syncPort)
	if err != nil {
		//log.Error("start sync listening fail")
		return err
	}

	err = ns.startConsListening(consPort)
	if err != nil {
		return err
	}

	return nil
}

func (ns *NetSubServer) startSyncListening(port uint16) error {
	var err error
	ns.synclistener, err = createListener(port)
	if err != nil {
		msg := fmt.Sprintf("failed to create sync listener err: %s", err.Error())
		P2PLog.Error(LOGTABLE_NETWORK, msg)
		return errors.New("failed to create sync listener")
	}

	go ns.startSyncAccept(ns.synclistener)
	msg := fmt.Sprintf("start listen on sync port %d", port)
	P2PLog.Info(LOGTABLE_NETWORK, msg)

	return nil
}

func (ns *NetSubServer) startConsListening(port uint16) error {
	var err error
	ns.conslistener, err = createListener(port)
	if err != nil {
		msg := fmt.Sprintf("failed to create cons listener err: %s", err.Error())
		P2PLog.Error(LOGTABLE_NETWORK, msg)
		return errors.New("failed to create cons listener")
	}

	go ns.startConsAccept(ns.conslistener)
	msg := fmt.Sprintf("start listen on consensus port %d", port)
	P2PLog.Info(LOGTABLE_NETWORK, msg)
	return nil
}

func (ns *NetSubServer) startSyncAccept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			msg := fmt.Sprintf("accept sync err:%s", err.Error())
			P2PLog.Warning(LOGTABLE_NETWORK, msg)
			return
		}
		if !ns.AddrValid(conn.RemoteAddr().String()) {
			P2PLog.Warningf(LOGTABLE_NETWORK, "remote %s not in reserved list, close it ", conn.RemoteAddr())
			conn.Close()
			continue
		}

		msg := fmt.Sprintf("remote sync node connect with remote:%s load:%s",
			conn.RemoteAddr(), conn.LocalAddr())
		P2PLog.Info(LOGTABLE_NETWORK, msg)

		if ns.IsAddrInInConnRecord(conn.RemoteAddr().String()) {
			conn.Close()
			continue
		}

		syncAddrCount := uint(ns.GetInConnRecordLen())
		if syncAddrCount >= config.Config.MaxConnInBound {
			msg := fmt.Sprintf("SyncAccept: total connections(%d) reach the max limit(%d), conn closed",
				syncAddrCount, config.Config.MaxConnInBound)
			P2PLog.Warning(LOGTABLE_NETWORK, msg)
			conn.Close()
			continue
		}

		remoteIp, err := common.ParseIPAddr(conn.RemoteAddr().String())
		if err != nil {
			msg := fmt.Sprintf("parse ip err:%s ", err.Error())
			P2PLog.Warning(LOGTABLE_NETWORK, msg)
			conn.Close()
			continue
		}
		connNum := ns.GetIpCountInInConnRecord(remoteIp)
		if connNum >= config.Config.MaxConnInBoundForSingleIP {
			msg := fmt.Sprintf("SyncAccept: connections(%d) with ip(%s) has reach the max limit(%d), "+
				"conn closed", connNum, remoteIp, config.Config.MaxConnInBoundForSingleIP)
			P2PLog.Warning(LOGTABLE_NETWORK, msg)
			conn.Close()
			continue
		}

		remotePeer := peer.NewPeer()
		addr := conn.RemoteAddr().String()
		ns.AddInConnRecord(addr)

		ns.AddPeerSyncAddress(addr, remotePeer)

		remotePeer.SyncLink.SetAddr(addr)
		remotePeer.SyncLink.SetConn(conn)
		remotePeer.AttachSyncChan(ns.SyncChan)
		go remotePeer.SyncLink.Rx()
	}
}

func (ns *NetSubServer) startConsAccept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			msg := fmt.Sprintf("accept cons err:%s", err.Error())
			P2PLog.Warning(LOGTABLE_NETWORK, msg)
			return
		}
		if !ns.AddrValid(conn.RemoteAddr().String()) {
			//	log.Warnf("remote %s not in reserved list, close it ", conn.RemoteAddr())
			conn.Close()
			continue
		}

		msg := fmt.Sprintf("remote cons node connect with remote:%s load:%s",
			conn.RemoteAddr(), conn.LocalAddr())
		P2PLog.Info(LOGTABLE_NETWORK, msg)

		remoteIp, err := common.ParseIPAddr(conn.RemoteAddr().String())
		if err != nil {
			msg := fmt.Sprintf("parse ip err:%s ", err.Error())
			P2PLog.Warning(LOGTABLE_NETWORK, msg)
			conn.Close()
			continue
		}
		if !ns.IsIPInInConnRecord(remoteIp) {
			conn.Close()
			continue
		}

		remotePeer := peer.NewPeer()
		addr := conn.RemoteAddr().String()
		ns.AddPeerConsAddress(addr, remotePeer)

		remotePeer.ConsLink.SetAddr(addr)
		remotePeer.ConsLink.SetConn(conn)
		remotePeer.AttachConsChan(ns.ConsChan)
		go remotePeer.ConsLink.Rx()
	}
}

func (ns *NetSubServer) AddOutConnectingList(addr string) (added bool) {
	ns.ConnectingNodes.Lock()
	defer ns.ConnectingNodes.Unlock()
	for _, a := range ns.ConnectingAddrs {
		if strings.Compare(a, addr) == 0 {
			return false
		}
	}
	ns.ConnectingAddrs = append(ns.ConnectingAddrs, addr)
	return true
}

func (ns *NetSubServer) RemoveFromConnectingList(addr string) {
	ns.ConnectingNodes.Lock()
	defer ns.ConnectingNodes.Unlock()
	addrs := ns.ConnectingAddrs[:0]
	for _, a := range ns.ConnectingAddrs {
		if a != addr {
			addrs = append(addrs, a)
		}
	}
	ns.ConnectingAddrs = addrs
}

func (ns *NetSubServer) GetOutConnectingListLen() (count uint) {
	ns.ConnectingNodes.RLock()
	defer ns.ConnectingNodes.RUnlock()
	return uint(len(ns.ConnectingAddrs))
}

func (ns *NetSubServer) IsAddrFromConnecting(addr string) bool {
	ns.ConnectingNodes.Lock()
	defer ns.ConnectingNodes.Unlock()
	for _, a := range ns.ConnectingAddrs {
		if strings.Compare(a, addr) == 0 {
			return true
		}
	}
	return false
}

func (ns *NetSubServer) GetPeerFromAddr(addr string) *peer.Peer {
	var p *peer.Peer
	ns.PeerAddrMap.RLock()
	defer ns.PeerAddrMap.RUnlock()

	p, ok := ns.PeerSyncAddress[addr]
	if ok {
		return p
	}
	p, ok = ns.PeerConsAddress[addr]
	if ok {
		return p
	}
	return nil
}

func (ns *NetSubServer) IsNbrPeerAddr(addr string, isConsensus bool) bool {
	var addrNew string
	ns.Np.RLock()
	defer ns.Np.RUnlock()
	for _, p := range ns.Np.List {
		if p.GetSyncState() == common.HAND || p.GetSyncState() == common.HAND_SHAKE ||
			p.GetSyncState() == common.ESTABLISH {
			if isConsensus {
				addrNew = p.ConsLink.GetAddr()
			} else {
				addrNew = p.SyncLink.GetAddr()
			}
			if strings.Compare(addrNew, addr) == 0 {
				return true
			}
		}
	}
	return false
}

func (ns *NetSubServer) AddPeerSyncAddress(addr string, p *peer.Peer) {
	ns.PeerAddrMap.Lock()
	defer ns.PeerAddrMap.Unlock()
	ns.PeerSyncAddress[addr] = p
}

func (ns *NetSubServer) AddPeerConsAddress(addr string, p *peer.Peer) {
	ns.PeerAddrMap.Lock()
	defer ns.PeerAddrMap.Unlock()
	ns.PeerConsAddress[addr] = p
}

func (ns *NetSubServer) RemovePeerSyncAddress(addr string) {
	ns.PeerAddrMap.Lock()
	defer ns.PeerAddrMap.Unlock()
	if _, ok := ns.PeerSyncAddress[addr]; ok {
		delete(ns.PeerSyncAddress, addr)
	}
}

func (ns *NetSubServer) RemovePeerConsAddress(addr string) {
	ns.PeerAddrMap.Lock()
	defer ns.PeerAddrMap.Unlock()
	if _, ok := ns.PeerConsAddress[addr]; ok {
		delete(ns.PeerConsAddress, addr)
	}
}

func (ns *NetSubServer) GetPeerSyncAddressCount() (count uint) {
	ns.PeerAddrMap.RLock()
	defer ns.PeerAddrMap.RUnlock()
	return uint(len(ns.PeerSyncAddress))
}

func (ns *NetSubServer) AddInConnRecord(addr string) {
	ns.inConnRecord.Lock()
	defer ns.inConnRecord.Unlock()
	for _, a := range ns.inConnRecord.InConnectingAddrs {
		if strings.Compare(a, addr) == 0 {
			return
		}
	}
	ns.inConnRecord.InConnectingAddrs = append(ns.inConnRecord.InConnectingAddrs, addr)
}

func (ns *NetSubServer) IsAddrInInConnRecord(addr string) bool {
	ns.inConnRecord.RLock()
	defer ns.inConnRecord.RUnlock()
	for _, a := range ns.inConnRecord.InConnectingAddrs {
		if strings.Compare(a, addr) == 0 {
			return true
		}
	}
	return false
}

func (ns *NetSubServer) IsIPInInConnRecord(ip string) bool {
	ns.inConnRecord.RLock()
	defer ns.inConnRecord.RUnlock()
	var ipRecord string
	for _, addr := range ns.inConnRecord.InConnectingAddrs {
		ipRecord, _ = common.ParseIPAddr(addr)
		if 0 == strings.Compare(ipRecord, ip) {
			return true
		}
	}
	return false
}

func (ns *NetSubServer) RemoveFromInConnRecord(addr string) {
	ns.inConnRecord.Lock()
	defer ns.inConnRecord.Unlock()
	addrs := []string{}
	for _, a := range ns.inConnRecord.InConnectingAddrs {
		if strings.Compare(a, addr) != 0 {
			addrs = append(addrs, a)
		}
	}
	ns.inConnRecord.InConnectingAddrs = addrs
}

func (ns *NetSubServer) GetInConnRecordLen() int {
	ns.inConnRecord.RLock()
	defer ns.inConnRecord.RUnlock()
	return len(ns.inConnRecord.InConnectingAddrs)
}

func (ns *NetSubServer) GetIpCountInInConnRecord(ip string) uint {
	ns.inConnRecord.RLock()
	defer ns.inConnRecord.RUnlock()
	var count uint
	var ipRecord string
	for _, addr := range ns.inConnRecord.InConnectingAddrs {
		ipRecord, _ = common.ParseIPAddr(addr)
		if 0 == strings.Compare(ipRecord, ip) {
			count++
		}
	}
	return count
}

func (ns *NetSubServer) AddOutConnRecord(addr string) {
	ns.outConnRecord.Lock()
	defer ns.outConnRecord.Unlock()
	for _, a := range ns.outConnRecord.OutConnectingAddrs {
		if strings.Compare(a, addr) == 0 {
			return
		}
	}
	ns.outConnRecord.OutConnectingAddrs = append(ns.outConnRecord.OutConnectingAddrs, addr)
}

func (ns *NetSubServer) IsAddrInOutConnRecord(addr string) bool {
	ns.outConnRecord.RLock()
	defer ns.outConnRecord.RUnlock()
	for _, a := range ns.outConnRecord.OutConnectingAddrs {
		if strings.Compare(a, addr) == 0 {
			return true
		}
	}
	return false
}

func (ns *NetSubServer) RemoveFromOutConnRecord(addr string) {
	ns.outConnRecord.Lock()
	defer ns.outConnRecord.Unlock()
	addrs := []string{}
	for _, a := range ns.outConnRecord.OutConnectingAddrs {
		if strings.Compare(a, addr) != 0 {
			addrs = append(addrs, a)
		}
	}
	ns.outConnRecord.OutConnectingAddrs = addrs
}

func (ns *NetSubServer) GetOutConnRecordLen() int {
	ns.outConnRecord.RLock()
	defer ns.outConnRecord.RUnlock()
	return len(ns.outConnRecord.OutConnectingAddrs)
}

func (ns *NetSubServer) AddrValid(addr string) bool {
	//if config.DefConfig.P2PNode.ReservedPeersOnly && len(config.DefConfig.P2PNode.ReservedCfg.ReservedPeers) > 0 {
	//	for _, ip := range config.DefConfig.P2PNode.ReservedCfg.ReservedPeers {
	//		if strings.HasPrefix(addr, ip) {
	//			log.Info("found reserved peer :", addr)
	//			return true
	//		}
	//	}
	//	return false
	//}
	return true
}

func (ns *NetSubServer) IsOwnAddress(addr string) bool {
	if addr == ns.OwnAddress {
		return true
	}
	return false
}

func (ns *NetSubServer) SetOwnAddress(addr string) {
	if addr != ns.OwnAddress {
		//log.Infof("set own address %s", addr)
		ns.OwnAddress = addr
	}

}

func createListener(port uint16) (net.Listener, error) {
	var listener net.Listener
	var err error

	isTls := config.Config.IsPeersTLS
	if isTls {
		listener, err = initTlsListen(port)
		if err != nil {
			//log.Error("initTlslisten failed")
			return nil, errors.New("initTlslisten failed")
		}
	} else {
		listener, err = initNonTlsListen(port)
		if err != nil {
			//log.Error("initNonTlsListen failed")
			return nil, errors.New("initNonTlsListen failed")
		}
	}
	return listener, nil
}

func nonTLSDial(addr string) (net.Conn, error) {
	//log.Debug()
	conn, err := net.DialTimeout("tcp", addr, time.Second*common.DIAL_TIMEOUT)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func TLSDial(nodeAddr string) (net.Conn, error) {
	//todo
	return nil, nil
}

func initNonTlsListen(port uint16) (net.Listener, error) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		//	log.Error("Error listening\n", err.Error())
		return nil, err
	}
	return listener, nil
}

func initTlsListen(port uint16) (net.Listener, error) {
	//todo
	return nil, nil
}
