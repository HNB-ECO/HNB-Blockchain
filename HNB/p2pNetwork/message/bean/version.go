package bean

import (
	"HNB/p2pNetwork/common"
	"bytes"
	"encoding/binary"
	"errors"
)

type VersionPayload struct {
	Version      uint32
	Services     uint64
	TimeStamp    int64
	SyncPort     uint16
	HttpInfoPort uint16
	ConsPort     uint16
	Cap          [32]byte
	Nonce        uint64
	StartHeight  uint64
	Relay        uint8
	IsConsensus  bool
}

type Version struct {
	P VersionPayload
}

func (this Version) Serialization() ([]byte, error) {
	p := bytes.NewBuffer([]byte{})
	err := binary.Write(p, binary.LittleEndian, &(this.P))
	if err != nil {
		return nil, errors.New("123")
	}

	return p.Bytes(), nil
}

func (this *Version) CmdType() string {
	return common.VERSION_TYPE
}

func (this *Version) Deserialization(p []byte) error {
	buf := bytes.NewBuffer(p)

	err := binary.Read(buf, binary.LittleEndian, &(this.P))
	if err != nil {
		return errors.New("456")
	}
	return nil
}
