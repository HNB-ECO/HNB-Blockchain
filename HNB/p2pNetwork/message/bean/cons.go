package bean

import (
	"HNB/p2pNetwork/common"
)

type ConsMsg struct {
	Msg []byte
}

//Serialize message payload
func (this ConsMsg) Serialization() ([]byte, error) {
	return this.Msg, nil
}

func (this *ConsMsg) CmdType() string {
	return common.CONS_TYPE
}

//Deserialize message payload
func (this *ConsMsg) Deserialization(p []byte) error {
	this.Msg = p
	return nil
}
