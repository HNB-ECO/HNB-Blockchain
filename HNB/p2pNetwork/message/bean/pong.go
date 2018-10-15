package bean

import (
	"bytes"
	"errors"
	"HNB/p2pNetwork/common"
	"encoding/binary"
)

type Pong struct {
	Height uint64
}

//Serialize message payload
func (this Pong) Serialization() ([]byte, error) {
	p := bytes.NewBuffer(nil)
	err := binary.Write(p, binary.LittleEndian, &(this.Height))
	if err != nil {
		return nil, errors.New("123")
	}

	return p.Bytes(), nil
}

func (this Pong) CmdType() string {
	return common.PONG_TYPE
}

//Deserialize message payload
func (this *Pong) Deserialization(p []byte) error {

	var height uint64
	buf := bytes.NewBuffer(p)
	err := binary.Read(buf, binary.LittleEndian, &(height))
	if err != nil {
		return errors.New("456")
	}
	this.Height = height
	return nil
}
