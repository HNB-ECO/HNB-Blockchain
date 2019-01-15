package utils

import (
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	msgCommon "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/message/bean"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/server"
)

var P2PLog logging.LogModule

const (
	LOGTABLE_NETWORK string = "network"
)

type MessageHandler func(data *bean.MsgPayload, p2p server.P2P, args ...interface{})

type MessageRouter struct {
	msgHandlers  map[string]MessageHandler
	RecvSyncChan chan *bean.MsgPayload
	RecvConsChan chan *bean.MsgPayload
	stopSyncCh   chan bool
	stopConsCh   chan bool
	p2p          server.P2P
	txFunc       msgCommon.NotifyFunc
	syncFunc     msgCommon.NotifyFunc
	consFunc     msgCommon.NotifyFunc
}

func NewMsgRouter(p2p server.P2P) *MessageRouter {
	P2PLog = logging.GetLogIns()
	msgRouter := &MessageRouter{}
	msgRouter.init(p2p)
	return msgRouter
}

func (mr *MessageRouter) RegisterTxNotify(nf msgCommon.NotifyFunc) {
	mr.txFunc = nf
}

func (mr *MessageRouter) RegisterSyncNotify(nf msgCommon.NotifyFunc) {
	mr.syncFunc = nf
}

func (mr *MessageRouter) RegisterConsNotify(nf msgCommon.NotifyFunc) {
	mr.consFunc = nf
}

func (mr *MessageRouter) init(p2p server.P2P) {
	mr.msgHandlers = make(map[string]MessageHandler)
	mr.RecvSyncChan = p2p.GetMsgChan(false)
	mr.RecvConsChan = p2p.GetMsgChan(true)
	mr.stopSyncCh = make(chan bool)
	mr.stopConsCh = make(chan bool)
	mr.p2p = p2p

	mr.RegisterHandler(msgCommon.VERSION_TYPE, VersionHandle)
	mr.RegisterHandler(msgCommon.VERACK_TYPE, VerAckHandle)
	mr.RegisterHandler(msgCommon.GetADDR_TYPE, AddrReqHandle)
	mr.RegisterHandler(msgCommon.ADDR_TYPE, AddrHandle)
	mr.RegisterHandler(msgCommon.PING_TYPE, PingHandle)
	mr.RegisterHandler(msgCommon.PONG_TYPE, PongHandle)
	mr.RegisterHandler(msgCommon.TEST_NETWORK, NetHandle)
	mr.RegisterHandler(msgCommon.DISCONNECT_TYPE, DisconnectHandle)

	mr.RegisterHandler(msgCommon.TX_TYPE, mr.TxHandle)
	mr.RegisterHandler(msgCommon.SYNC_TYPE, mr.SyncHandle)
	mr.RegisterHandler(msgCommon.CONS_TYPE, mr.ConsHandle)
}

func (mr *MessageRouter) RegisterHandler(key string,
	handler MessageHandler) {
	mr.msgHandlers[key] = handler
}

func (mr *MessageRouter) UnRegisterHandler(key string) {
	delete(mr.msgHandlers, key)
}

func (mr *MessageRouter) Start() {
	go mr.rxMsg(mr.RecvSyncChan, mr.stopSyncCh)
	go mr.rxMsg(mr.RecvConsChan, mr.stopConsCh)
}

func (mr *MessageRouter) rxMsg(channel chan *bean.MsgPayload,
	stopCh chan bool) {
	for {
		select {
		case data, ok := <-channel:
			if ok {
				msgType := data.Payload.CmdType()
				sp := fmt.Sprintf("recv msg %v", msgType)
				P2PLog.Debug(LOGTABLE_NETWORK, sp)
				handler, ok := mr.msgHandlers[msgType]
				if ok {
					handler(data, mr.p2p, mr)
				} else {
					//log.Info("unknown message handler for the msg: ",
					//	msgType)
				}
			}
		case <-stopCh:
			return
		}
	}
}

func (mr *MessageRouter) Stop() {

	if mr.stopSyncCh != nil {
		mr.stopSyncCh <- true
	}
	if mr.stopConsCh != nil {
		mr.stopConsCh <- true
	}
}
