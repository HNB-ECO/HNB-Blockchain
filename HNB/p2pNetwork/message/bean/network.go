package bean

import (
	"encoding/json"
	"errors"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
)

type TestNetwork struct {
	Msg string
}

//Serialize message payload
func (this TestNetwork) Serialization() ([]byte, error) {
	//p := bytes.NewBuffer([]byte{})
	//err := binary.Write(p, binary.LittleEndian, &(this.Msg))
	//if err != nil {
	//	return nil, errors.New("123")
	//}
	//
	//return p.Bytes(), nil

	m, err := json.Marshal(this.Msg)
	if err != nil {
		return nil, errors.New("123")
	}
	return m, nil
}

func (this *TestNetwork) CmdType() string {
	return common.TEST_NETWORK
}

//Deserialize message payload
func (this *TestNetwork) Deserialization(p []byte) error {
	var msg string
	err := json.Unmarshal(p, &msg)
	if err != nil {
		return errors.New("456")
	}
	this.Msg = msg
	return nil
}
