package peer

import (
	conn "HNB/p2pNetwork/receive"
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/bean"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var P2PLog logging.LogModule

const (
	LOGTABLE_NETWORK string = "network"
)

type PeerCom struct {
	id           uint64
	version      uint32
	services     uint64
	relay        bool
	httpInfoPort uint16
	syncPort     uint16
	consPort     uint16
	height       uint64
}

func (pc *PeerCom) SetID(id uint64) {
	pc.id = id
}

func (pc *PeerCom) GetID() uint64 {
	return pc.id
}

func (pc *PeerCom) SetVersion(version uint32) {
	pc.version = version
}

func (pc *PeerCom) GetVersion() uint32 {
	return pc.version
}

func (pc *PeerCom) SetServices(services uint64) {
	pc.services = services
}

func (pc *PeerCom) GetServices() uint64 {
	return pc.services
}

func (pc *PeerCom) SetRelay(relay bool) {
	pc.relay = relay
}

func (pc *PeerCom) GetRelay() bool {
	return pc.relay
}

func (pc *PeerCom) SetSyncPort(port uint16) {
	pc.syncPort = port
}

func (pc *PeerCom) GetSyncPort() uint16 {
	return pc.syncPort
}

func (pc *PeerCom) SetConsPort(port uint16) {
	pc.consPort = port
}

func (pc *PeerCom) GetConsPort() uint16 {
	return pc.consPort
}

func (pc *PeerCom) SetHttpInfoPort(port uint16) {
	pc.httpInfoPort = port
}

func (pc *PeerCom) GetHttpInfoPort() uint16 {
	return pc.httpInfoPort
}

func (pc *PeerCom) SetHeight(height uint64) {
	pc.height = height
}

func (pc *PeerCom) GetHeight() uint64 {
	return pc.height
}

type Peer struct {
	base      PeerCom
	cap       [32]byte
	SyncLink  *conn.ConnInfo
	ConsLink  *conn.ConnInfo
	syncState uint32
	consState uint32
	txnCnt    uint64
	rxTxnCnt  uint64
	connLock  sync.RWMutex
}

func NewPeer() *Peer {
	p := &Peer{
		syncState: common.INIT,
		consState: common.INIT,
	}
	p.SyncLink = conn.NewLink()
	p.ConsLink = conn.NewLink()
	runtime.SetFinalizer(p, rmPeer)
	P2PLog = logging.GetLogIns()
	return p
}

func rmPeer(p *Peer) {
	//log.Debug(fmt.Sprintf("Remove unused peer: %d", p.GetID()))
}

func (pc *Peer) DumpInfo() {

}

func (pc *Peer) GetVersion() uint32 {
	return pc.base.GetVersion()
}

func (pc *Peer) GetHeight() uint64 {
	return pc.base.GetHeight()
}

func (pc *Peer) SetHeight(height uint64) {
	pc.base.SetHeight(height)
}

func (pc *Peer) GetConsConn() *conn.ConnInfo {
	return pc.ConsLink
}

func (pc *Peer) SetConsConn(consLink *conn.ConnInfo) {
	pc.ConsLink = consLink
}

func (pc *Peer) GetSyncState() uint32 {
	return pc.syncState
}

func (pc *Peer) SetSyncState(state uint32) {
	atomic.StoreUint32(&(pc.syncState), state)
}

func (pc *Peer) GetConsState() uint32 {
	return pc.consState
}

func (pc *Peer) SetConsState(state uint32) {
	atomic.StoreUint32(&(pc.consState), state)
}

func (pc *Peer) GetSyncPort() uint16 {
	return pc.SyncLink.GetPort()
}

func (pc *Peer) GetConsPort() uint16 {
	return pc.ConsLink.GetPort()
}

func (pc *Peer) SetConsPort(port uint16) {
	pc.ConsLink.SetPort(port)
}

func (pc *Peer) SendToSync(msg bean.Message) error {
	if pc.SyncLink != nil && pc.SyncLink.Valid() {
		return pc.SyncLink.Tx(msg)
	}
	return errors.New("sync link invalid")
}

func (pc *Peer) SendToCons(msg bean.Message) error {
	if pc.ConsLink != nil && pc.ConsLink.Valid() {
		return pc.ConsLink.Tx(msg)
	}
	return errors.New("cons link invalid")
}

func (pc *Peer) CloseSync() {
	pc.SetSyncState(common.INACTIVITY)
	conn := pc.SyncLink.GetConn()
	pc.connLock.Lock()
	if conn != nil {
		conn.Close()
	}
	pc.connLock.Unlock()
}

func (pc *Peer) CloseCons() {
	pc.SetConsState(common.INACTIVITY)
	conn := pc.ConsLink.GetConn()
	pc.connLock.Lock()
	if conn != nil {
		conn.Close()

	}
	pc.connLock.Unlock()
}

func (pc *Peer) GetID() uint64 {
	return pc.base.GetID()
}

func (pc *Peer) GetRelay() bool {
	return pc.base.GetRelay()
}

func (pc *Peer) GetServices() uint64 {
	return pc.base.GetServices()
}

func (pc *Peer) GetTimeStamp() int64 {
	return pc.SyncLink.GetRXTime().UnixNano()
}

func (pc *Peer) GetContactTime() time.Time {
	return pc.SyncLink.GetRXTime()
}

func (pc *Peer) GetAddr() string {
	return pc.SyncLink.GetAddr()
}

func (pc *Peer) GetAddr16() ([16]byte, error) {
	var result [16]byte
	addrIp, err := common.ParseIPAddr(pc.GetAddr())
	if err != nil {
		return result, err
	}
	ip := net.ParseIP(addrIp).To16()
	if ip == nil {
		//log.Error("parse ip address error\n", pc.GetAddr())
		return result, errors.New("parse ip address error")
	}

	copy(result[:], ip[:16])
	return result, nil
}

//AttachSyncChan set msg chan to sync link
func (pc *Peer) AttachSyncChan(msgchan chan *bean.MsgPayload) {
	pc.SyncLink.SetChan(msgchan)
}

func (pc *Peer) AttachConsChan(msgchan chan *bean.MsgPayload) {
	pc.ConsLink.SetChan(msgchan)
}

func (pc *Peer) Send(msg bean.Message, isConsensus bool) error {

	sp := fmt.Sprintf("send msg %v,cons:%v", msg.CmdType(), isConsensus)
	P2PLog.Debug(LOGTABLE_NETWORK, sp)

	if isConsensus && pc.ConsLink.Valid() {
		return pc.SendToCons(msg)
	}
	return pc.SendToSync(msg)
}

func (pc *Peer) SetHttpInfoState(httpInfo bool) {
	if httpInfo {
		pc.cap[common.HTTP_INFO_FLAG] = 0x01
	} else {
		pc.cap[common.HTTP_INFO_FLAG] = 0x00
	}
}

func (pc *Peer) GetHttpInfoState() bool {
	return pc.cap[common.HTTP_INFO_FLAG] == 1
}

func (pc *Peer) GetHttpInfoPort() uint16 {
	return pc.base.GetHttpInfoPort()
}

func (pc *Peer) SetHttpInfoPort(port uint16) {
	pc.base.SetHttpInfoPort(port)
}

func (pc *Peer) UpdateInfo(t time.Time, version uint32, services uint64,
	syncPort uint16, consPort uint16, nonce uint64, relay uint8, height uint64) {

	pc.SyncLink.UpdateRXTime(t)
	pc.base.SetID(nonce)
	pc.base.SetVersion(version)
	pc.base.SetServices(services)
	pc.base.SetSyncPort(syncPort)
	pc.base.SetConsPort(consPort)
	pc.SyncLink.SetPort(syncPort)
	pc.ConsLink.SetPort(consPort)
	if relay == 0 {
		pc.base.SetRelay(false)
	} else {
		pc.base.SetRelay(true)
	}
	pc.SetHeight(uint64(height))
}
