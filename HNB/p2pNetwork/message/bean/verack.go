package bean

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
)

type VerACK struct {
	IsConsensus bool
}

func (this VerACK) Serialization() ([]byte, error) {
	p := bytes.NewBuffer([]byte{})
	err := binary.Write(p, binary.LittleEndian, &(this.IsConsensus))
	if err != nil {
		return nil, errors.New("123")
	}

	return p.Bytes(), nil
}

func (this VerACK) CmdType() string {
	return common.VERACK_TYPE
}

func (this *VerACK) Deserialization(p []byte) error {
	var isConsensus bool
	buf := bytes.NewBuffer(p)
	err := binary.Read(buf, binary.LittleEndian, &(isConsensus))
	if err != nil {
		return errors.New("789")
	}

	this.IsConsensus = isConsensus
	return nil
}
