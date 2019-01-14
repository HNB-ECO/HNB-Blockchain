package bean

import (
	"bytes"
	"encoding/binary"

	"errors"
	comm "github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
)

type Addr struct {
	NodeAddrs []comm.PeerAddr
}

func (this Addr) Serialization() ([]byte, error) {
	p := new(bytes.Buffer)
	num := uint64(len(this.NodeAddrs))
	err := binary.Write(p, binary.LittleEndian, num)
	if err != nil {
		return nil, errors.New("123")
	}

	err = binary.Write(p, binary.LittleEndian, this.NodeAddrs)
	if err != nil {
		return nil, errors.New("456")
	}

	return p.Bytes(), nil
}

func (this *Addr) CmdType() string {
	return comm.ADDR_TYPE
}

func (this *Addr) Deserialization(p []byte) error {
	buf := bytes.NewBuffer(p)

	var NodeCnt uint64
	err := binary.Read(buf, binary.LittleEndian, &NodeCnt)
	if NodeCnt > comm.MAX_ADDR_NODE_CNT {
		NodeCnt = comm.MAX_ADDR_NODE_CNT
	}
	if err != nil {
		return errors.New("789")
	}

	for i := 0; i < int(NodeCnt); i++ {
		var addr comm.PeerAddr
		err := binary.Read(buf, binary.LittleEndian, &addr)
		if err != nil {
			return errors.New("JQK")
		}
		this.NodeAddrs = append(this.NodeAddrs, addr)
	}
	return nil
}
