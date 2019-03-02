package bean

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
)

type SyncMsg struct {
	Msg []byte
}

//Serialize message payload
func (this SyncMsg) Serialization() ([]byte, error) {
	return this.Msg, nil
}

func (this *SyncMsg) CmdType() string {
	return common.SYNC_TYPE
}

//Deserialize message payload
func (this *SyncMsg) Deserialization(p []byte) error {
	this.Msg = p
	return nil
}
