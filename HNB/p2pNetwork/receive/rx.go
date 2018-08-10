package receive

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"time"
	"HNB/p2pNetwork/message/bean"
	"HNB/p2pNetwork/common"
	"HNB/logging"
)

type ConnInfo struct {
	id       uint64
	addr     string
	conn     net.Conn
	port     uint16
	time     time.Time
	recvChan chan *bean.MsgPayload
}

func NewLink() *ConnInfo {
	link := &ConnInfo{}
	return link
}

func (lk *ConnInfo) SetID(id uint64) {
	lk.id = id
}

func (lk *ConnInfo) GetID() uint64 {
	return lk.id
}

func (lk *ConnInfo) Valid() bool {
	return lk.conn != nil
}

func (lk *ConnInfo) SetChan(msgchan chan *bean.MsgPayload) {
	lk.recvChan = msgchan
}

func (lk *ConnInfo) GetAddr() string {
	return lk.addr
}

func (lk *ConnInfo) SetAddr(addr string) {
	lk.addr = addr
}

func (lk *ConnInfo) SetPort(p uint16) {
	lk.port = p
}

func (lk *ConnInfo) GetPort() uint16 {
	return lk.port
}

func (lk *ConnInfo) GetConn() net.Conn {
	return lk.conn
}

func (lk *ConnInfo) SetConn(conn net.Conn) {
	lk.conn = conn
}

func (lk *ConnInfo) UpdateRXTime(t time.Time) {
	lk.time = t
}

func (lk *ConnInfo) GetRXTime() time.Time {
	return lk.time
}

func (lk *ConnInfo) Rx() {
	reader := bufio.NewReaderSize(lk.conn, common.MAX_BUF_LEN)

	for {
		msg, err := bean.ReadMessage(reader)
		if err != nil {
			logging.GetLogIns().Error("rx", "read connection err: " + err.Error())
			break
		}

		t := time.Now()
		lk.UpdateRXTime(t)
		lk.recvChan <- &bean.MsgPayload{
			Id:      lk.id,
			Addr:    lk.addr,
			Payload: msg,
		}

	}

	lk.disconnectNotify()
}

func (lk *ConnInfo) disconnectNotify() {
	lk.CloseConn()

	msg, _ := bean.MakeEmptyMessage(common.DISCONNECT_TYPE)
	discMsg := &bean.MsgPayload{
		Id:      lk.id,
		Addr:    lk.addr,
		Payload: msg,
	}

	lk.recvChan <- discMsg
}

func (lk *ConnInfo) CloseConn() {
	if lk.conn != nil {
		lk.conn.Close()
		lk.conn = nil
	}
}

func (lk *ConnInfo) Tx(msg bean.Message) error {
	conn := lk.conn
	if conn == nil {
		return errors.New("tx link invalid")
	}
	buf := bytes.NewBuffer(nil)
	err := bean.WriteMessage(buf, msg)
	if err != nil {
		return err
	}

	payload := buf.Bytes()
	nByteCnt := len(payload)

	nCount := nByteCnt / common.PER_SEND_LEN
	if nCount == 0 {
		nCount = 1
	}
	conn.SetWriteDeadline(time.Now().Add(time.Duration(nCount*common.WRITE_DEADLINE) * time.Second))
	_, err = conn.Write(payload)
	if err != nil {
		lk.disconnectNotify()
		return err
	}

	return nil
}
