package bean

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
)

type Disconnected struct{}

func (this Disconnected) Serialization() ([]byte, error) {
	return nil, nil
}

func (this Disconnected) CmdType() string {
	return common.DISCONNECT_TYPE
}

//Deserialize message payload
func (this *Disconnected) Deserialization(p []byte) error {
	return nil
}
