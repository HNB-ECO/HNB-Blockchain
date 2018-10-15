package bean

import (
	"HNB/p2pNetwork/common"
)

type AddrReq struct{}

func (this AddrReq) Serialization() ([]byte, error) {
	return nil, nil
}

func (this *AddrReq) CmdType() string {
	return common.GetADDR_TYPE
}

//Deserialize message payload
func (this *AddrReq) Deserialization(p []byte) error {
	return nil
}
