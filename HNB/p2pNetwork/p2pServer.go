package p2pNetwork

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	msgtypes "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/bean"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/reqMsg"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/utils"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/peer"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/server"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

var P2PLog logging.LogModule

const (
	LOGTABLE_NETWORK string = "network"
)

type PPNetServer struct {
	network   server.P2P
	msgRouter *utils.MessageRouter
	ReconnectAddrs
	recentPeers    []string
	quitSyncRecent chan bool
	quitOnline     chan bool
	quitHeartBeat  chan bool
}

type ReconnectAddrs struct {
	sync.RWMutex
	RetryAddrs map[string]int
}

var P2PIns *PPNetServer

func NewServer() *PPNetServer {
	P2PLog = logging.GetLogIns()
	P2PLog.Info(LOGTABLE_NETWORK, "ready start p2p server")

	n := NewNetServer()

	p := &PPNetServer{
		network: n,
	}

	p.msgRouter = utils.NewMsgRouter(p.network)
	p.recentPeers = make([]string, common.RECENT_LIMIT)
	p.quitSyncRecent = make(chan bool)
	p.quitOnline = make(chan bool)
	p.quitHeartBeat = make(chan bool)

	P2PIns = p

	return p
}

func (pps *PPNetServer) RegisterTxNotify() uint32 {
	return pps.network.GetConnectionCnt()
}

func (pps *PPNetServer) RegisterSyncNotify() uint32 {
	return pps.network.GetConnectionCnt()
}

func (pps *PPNetServer) RegisterConsNotify() uint32 {
	return pps.network.GetConnectionCnt()
}

func (pps *PPNetServer) GetConnectionCnt() uint32 {
	return pps.network.GetConnectionCnt()
}

func (pps *PPNetServer) Start() error {
	if pps.network != nil {
		pps.network.Start()
	} else {
		return errors.New("p2p network invalid")
	}
	if pps.msgRouter != nil {
		pps.msgRouter.Start()
	} else {
		return errors.New("p2p msg router invalid")
	}
	pps.tryRecentPeers()
	go pps.connectSeedService()
	go pps.syncUpRecentPeers()
	go pps.keepOnlineService()
	go pps.heartBeatService()
	return nil
}

func (pps *PPNetServer) Stop() {
	pps.network.Halt()
	pps.quitSyncRecent <- true
	pps.quitOnline <- true
	pps.quitHeartBeat <- true
	pps.msgRouter.Stop()
}

func (pps *PPNetServer) GetNetWork() server.P2P {
	return pps.network
}

func (pps *PPNetServer) GetPort() (uint16, uint16) {
	return pps.network.GetSyncPort(), pps.network.GetConsPort()
}

func (pps *PPNetServer) GetVersion() uint32 {
	return pps.network.GetVersion()
}

func (pps *PPNetServer) GetNeighborAddrs() []common.PeerAddr {
	return pps.network.GetNeighborAddrs()
}

//func (pps *PPNetServer) Xmit(msg string, isConsensus bool){
//	m := msgpack.NewTestNet(msg)
//	pps.network.Xmit(m, isConsensus)
//}

func (pps *PPNetServer) Send(p *peer.Peer, msg msgtypes.Message,
	isConsensus bool) error {
	if pps.network.IsPeerEstablished(p) {
		return pps.network.Send(p, msg, isConsensus)
	}
	//log.Errorf("PPNetServer send to a not ESTABLISH peer %d",
	//	p.GetID())

	return errors.New("send to a not ESTABLISH peer")
}

func (pps *PPNetServer) GetID() uint64 {
	return pps.network.GetID()
}

func (pps *PPNetServer) GetConnectionState() uint32 {
	return common.INIT
}

func (pps *PPNetServer) GetTime() int64 {
	return pps.network.GetTime()
}

func (pps *PPNetServer) WaitForPeersStart() {
	//periodTime := time.Minute//config.DEFAULT_GEN_BLOCK_TIME / common.UPDATE_RATE_PER_BLOCK
	//for {
	//	//log.Info("Wait for minimum connection...")
	//	if pps.reachMinConnection() {
	//		break
	//	}
	//
	//	<-time.After(time.Second * (time.Duration(periodTime)))
	//}
}

//链接发现种子
func (pps *PPNetServer) connectSeeds() {
	seedNodes := make([]string, 0)
	pList := make([]*peer.Peer, 0)

	for _, n := range config.Config.SeedList {
		//获取IP地址
		ip, err := common.ParseIPAddr(n)
		if err != nil {
			P2PLog.Warningf(LOGTABLE_NETWORK, "seed peer %s address format is wrong", n)
			continue
		}

		//host 主机与ip映射
		ns, err := net.LookupHost(ip)
		if err != nil {
			P2PLog.Warningf(LOGTABLE_NETWORK, "resolve err: %s", err.Error())
			continue
		}

		//获取端口号
		port, err := common.ParseIPPort(n)
		if err != nil {
			P2PLog.Warningf(LOGTABLE_NETWORK, "seed peer %s address format is wrong", n)
			continue
		}

		seedNodes = append(seedNodes, ns[0]+port)
	}

	for _, nodeAddr := range seedNodes {
		var ip net.IP
		np := pps.network.GetNp()
		P2PLog.Infof(LOGTABLE_NETWORK, "process seedNode %v", nodeAddr)
		np.Lock()
		for _, tn := range np.List {
			ipAddr, _ := tn.GetAddr16()
			ip = ipAddr[:]
			addrString := ip.To16().String() + ":" +
				strconv.Itoa(int(tn.GetSyncPort()))
			if nodeAddr == addrString && tn.GetSyncState() == common.ESTABLISH {
				pList = append(pList, tn)
			}
		}
		np.Unlock()
	}

	if len(pList) > 0 {
		rand.Seed(time.Now().UnixNano())
		index := rand.Intn(len(pList))
		pps.reqNbrList(pList[index])
	} else { //not found
		for _, nodeAddr := range seedNodes {
			go pps.network.Connect(nodeAddr, false)
		}
	}
}

func (pps *PPNetServer) getNode(id uint64) *peer.Peer {
	return pps.network.GetPeer(id)
}

func (pps *PPNetServer) retryInactivePeer() {
	np := pps.network.GetNp()
	np.Lock()
	var ip net.IP
	neighborPeers := make(map[uint64]*peer.Peer)
	for _, p := range np.List {
		addr, _ := p.GetAddr16()
		ip = addr[:]
		nodeAddr := ip.To16().String() + ":" +
			strconv.Itoa(int(p.GetSyncPort()))
		if p.GetSyncState() == common.INACTIVITY {
			//log.Infof(" try reconnect %s", nodeAddr)
			pps.addToRetryList(nodeAddr)
			p.CloseSync()
			p.CloseCons()
		} else {
			pps.removeFromRetryList(nodeAddr)
			neighborPeers[p.GetID()] = p
		}
	}

	np.List = neighborPeers
	np.Unlock()

	connCount := uint(pps.network.GetOutConnRecordLen())
	if connCount >= config.Config.MaxConnOutBound {
		//log.Warnf("P2PLog.Warningf(LOGTABLE_NETWORK,: out connections(%d) reach the max limit(%d)", connCount,
		//	config.DefConfig.P2PNode.MaxConnOutBound)
		return
	}

	if len(pps.RetryAddrs) > 0 {
		pps.ReconnectAddrs.Lock()

		list := make(map[string]int)
		addrs := make([]string, 0, len(pps.RetryAddrs))
		for addr, v := range pps.RetryAddrs {
			v += 1
			addrs = append(addrs, addr)
			if v < common.MAX_RETRY_COUNT {
				list[addr] = v
			}
			if v >= common.MAX_RETRY_COUNT {
				pps.network.RemoveFromConnectingList(addr)
				remotePeer := pps.network.GetPeerFromAddr(addr)
				if remotePeer != nil {
					if remotePeer.SyncLink.GetAddr() == addr {
						pps.network.RemovePeerSyncAddress(addr)
						pps.network.RemovePeerConsAddress(addr)
					}
					if remotePeer.ConsLink.GetAddr() == addr {
						pps.network.RemovePeerConsAddress(addr)
					}
					pps.network.DelNbrNode(remotePeer.GetID())
				}
			}
		}

		pps.RetryAddrs = list
		pps.ReconnectAddrs.Unlock()
		for _, addr := range addrs {
			rand.Seed(time.Now().UnixNano())
			P2PLog.Infof(LOGTABLE_NETWORK, "Try to reconnect peer, peer addr is ", addr)
			<-time.After(time.Duration(rand.Intn(common.CONN_MAX_BACK)) * time.Millisecond)
			P2PLog.Infof(LOGTABLE_NETWORK, "Back off time`s up, start connect node")
			pps.network.Connect(addr, false)
		}

	}
}

func (pps *PPNetServer) connectSeedService() {
	t := time.NewTimer(time.Second * common.CONN_MONITOR)
	for {
		select {
		case <-t.C:
			pps.connectSeeds()
			t.Stop()
			t.Reset(time.Second * common.CONN_MONITOR)
		case <-pps.quitOnline:
			t.Stop()
			break
		}
	}
}

func (pps *PPNetServer) keepOnlineService() {
	t := time.NewTimer(time.Second * common.CONN_MONITOR)
	for {
		select {
		case <-t.C:
			pps.retryInactivePeer()
			t.Stop()
			t.Reset(time.Second * common.CONN_MONITOR)
		case <-pps.quitOnline:
			t.Stop()
			break
		}
	}
}

func (pps *PPNetServer) reqNbrList(p *peer.Peer) {
	msg := reqMsg.NewAddrReq()
	go pps.Send(p, msg, false)
}

func (pps *PPNetServer) heartBeatService() {
	var periodTime uint
	periodTime = 10
	t := time.NewTicker(time.Second * (time.Duration(periodTime)))

	for {
		select {
		case <-t.C:
			pps.ping()
			pps.timeout()
		case <-pps.quitHeartBeat:
			t.Stop()
			break
		}
	}
}

func (pps *PPNetServer) ping() {
	peers := pps.network.GetNeighbors()
	for _, p := range peers {
		if p.GetSyncState() == common.ESTABLISH {
			//height := pps.ledger.GetCurrentBlockHeight()
			height := 0
			ping := reqMsg.NewPingMsg(uint64(height))
			go pps.Send(p, ping, false)
		}
	}
}

func (pps *PPNetServer) timeout() {
	peers := pps.network.GetNeighbors()
	var periodTime uint
	periodTime = 10
	for _, p := range peers {
		if p.GetSyncState() == common.ESTABLISH {
			t := p.GetContactTime()
			if t.Before(time.Now().Add(-1 * time.Second *
				time.Duration(periodTime) * common.KEEPALIVE_TIMEOUT)) {
				//log.Warnf("keep alive timeout!!!lost remote peer %d - %s from %s", p.GetID(), p.SyncLink.GetAddr(), t.String())
				p.CloseSync()
				p.CloseCons()
			}
		}
	}
}

func (pps *PPNetServer) addToRetryList(addr string) {
	pps.ReconnectAddrs.Lock()
	defer pps.ReconnectAddrs.Unlock()
	if pps.RetryAddrs == nil {
		pps.RetryAddrs = make(map[string]int)
	}
	if _, ok := pps.RetryAddrs[addr]; ok {
		delete(pps.RetryAddrs, addr)
	}
	//alway set retry to 0
	pps.RetryAddrs[addr] = 0
}

func (pps *PPNetServer) removeFromRetryList(addr string) {
	pps.ReconnectAddrs.Lock()
	defer pps.ReconnectAddrs.Unlock()
	if len(pps.RetryAddrs) > 0 {
		if _, ok := pps.RetryAddrs[addr]; ok {
			delete(pps.RetryAddrs, addr)
		}
	}
}

func (pps *PPNetServer) tryRecentPeers() {
	//假设在当前路径
	if util.PathExists(common.RECENT_FILE_NAME) {
		buf, err := ioutil.ReadFile(common.RECENT_FILE_NAME)
		if err != nil {
			errMsg := fmt.Sprintf("read %s fail:%s, connect recent peers cancel", common.RECENT_FILE_NAME, err.Error())
			P2PLog.Warning(LOGTABLE_NETWORK, errMsg)
			return
		}

		err = json.Unmarshal(buf, &pps.recentPeers)
		if err != nil {
			errMsg := fmt.Sprintf("parse recent peer file fail: %s", err.Error())
			P2PLog.Warning(LOGTABLE_NETWORK, errMsg)
			return
		}
		if len(pps.recentPeers) > 0 {
			P2PLog.Infof(LOGTABLE_NETWORK, "try to connect recent peer")
		}
		for _, v := range pps.recentPeers {
			msg := fmt.Sprintf("try to connect recent peer %s", v)
			P2PLog.Info(LOGTABLE_NETWORK, msg)
			go pps.network.Connect(v, false)
		}

	}
}

func (pps *PPNetServer) syncUpRecentPeers() {
	periodTime := common.RECENT_TIMEOUT
	t := time.NewTicker(time.Second * (time.Duration(periodTime)))
	for {
		select {
		case <-t.C:
			pps.syncPeerAddr()
		case <-pps.quitSyncRecent:
			t.Stop()
			break
		}
	}
}

//syncPeerAddr compare snapshot of recent peer with current link,then persist the list
func (pps *PPNetServer) syncPeerAddr() {
	changed := false
	for i := 0; i < len(pps.recentPeers); i++ {
		p := pps.network.GetPeerFromAddr(pps.recentPeers[i])
		if p == nil || (p != nil && p.GetSyncState() != common.ESTABLISH) {
			pps.recentPeers = append(pps.recentPeers[:i], pps.recentPeers[i+1:]...)
			changed = true
			i--
		}
	}
	left := common.RECENT_LIMIT - len(pps.recentPeers)
	if left > 0 {
		np := pps.network.GetNp()
		np.Lock()
		var ip net.IP
		for _, p := range np.List {
			addr, _ := p.GetAddr16()
			ip = addr[:]
			nodeAddr := ip.To16().String() + ":" +
				strconv.Itoa(int(p.GetSyncPort()))
			found := false
			for i := 0; i < len(pps.recentPeers); i++ {
				if nodeAddr == pps.recentPeers[i] {
					found = true
					break
				}
			}
			if !found {
				pps.recentPeers = append(pps.recentPeers, nodeAddr)
				left--
				changed = true
				if left == 0 {
					break
				}
			}
		}
		np.Unlock()
	}
	if changed {
		buf, err := json.Marshal(pps.recentPeers)
		if err != nil {
			//log.Error("package recent peer fail: ", err)
			return
		}
		err = ioutil.WriteFile(common.RECENT_FILE_NAME, buf, os.ModePerm)
		if err != nil {
			//log.Error("write recent peer fail: ", err)
		}
	}
}
