
package bean

import (
	"bytes"
	"HNB/p2pNetwork/common"
	"encoding/binary"
	"errors"
)

type Ping struct {
	Height uint64
}

//Serialize message payload
func (this Ping) Serialization() ([]byte, error) {
	p := bytes.NewBuffer([]byte{})
	err := binary.Write(p, binary.LittleEndian, &(this.Height))
	if err != nil {
		return nil, errors.New("123")
	}

	return p.Bytes(), nil
}

func (this *Ping) CmdType() string {
	return common.PING_TYPE
}

//Deserialize message payload
func (this *Ping) Deserialization(p []byte) error {

	var height uint64
	buf := bytes.NewBuffer(p)
	err := binary.Read(buf, binary.LittleEndian, &(height))
	if err != nil {
		return errors.New("456")
	}
	this.Height = height
	return nil
}
