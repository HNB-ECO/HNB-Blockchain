package bean

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
)

type TxMsg struct {
	Msg []byte
}

//Serialize message payload
func (this TxMsg) Serialization() ([]byte, error) {
	return this.Msg, nil
}

func (this *TxMsg) CmdType() string {
	return common.TX_TYPE
}

//Deserialize message payload
func (this *TxMsg) Deserialization(p []byte) error {
	this.Msg = p
	return nil
}
