package bean

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork/common"
	"io"
)

type Message interface {
	Serialization() ([]byte, error)
	Deserialization([]byte) error
	CmdType() string
}

type MsgPayload struct {
	Id      uint64  //peer ID
	Addr    string  //link address
	Payload Message //Msg payload
}

type messageHeader struct {
	Magic    uint32
	CMD      [common.MSG_CMD_LEN]byte // The message type
	Length   uint32
	Checksum [common.CHECKSUM_LEN]byte
}

func readMessageHeader(reader io.Reader) (messageHeader, error) {
	msgh := messageHeader{}
	err := binary.Read(reader, binary.LittleEndian, &msgh)
	return msgh, err
}

func writeMessageHeader(writer io.Writer, msgh messageHeader) error {
	return binary.Write(writer, binary.LittleEndian, msgh)
}

func newMessageHeader(cmd string, length uint32, checksum [common.CHECKSUM_LEN]byte) messageHeader {
	msgh := messageHeader{}
	msgh.Magic = 1 //config.DefConfig.P2PNode.NetworkMagic
	copy(msgh.CMD[:], cmd)
	msgh.Checksum = checksum
	msgh.Length = length
	return msgh
}

func WriteMessage(writer io.Writer, msg Message) error {
	buf, err := msg.Serialization()
	if err != nil {
		return err
	}
	checksum := CheckSum(buf)

	hdr := newMessageHeader(msg.CmdType(), uint32(len(buf)), checksum)

	err = writeMessageHeader(writer, hdr)
	if err != nil {
		return err
	}

	_, err = writer.Write(buf)
	return err
}

func ReadMessage(reader io.Reader) (Message, error) {
	hdr, err := readMessageHeader(reader)
	if err != nil {
		return nil, err
	}

	//magic := 1//config.DefConfig.P2PNode.NetworkMagic
	//if hdr.Magic != magic {
	//	return nil, fmt.Errorf("unmatched magic number %d, expected %d", hdr.Magic, magic)
	//}

	if int(hdr.Length) > common.MAX_PAYLOAD_LEN {
		return nil, fmt.Errorf("Msg payload length:%d exceed max payload size: %d",
			hdr.Length, common.MAX_PAYLOAD_LEN)
	}

	buf := make([]byte, hdr.Length)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, err
	}

	checksum := CheckSum(buf)
	if checksum != hdr.Checksum {
		return nil, fmt.Errorf("message checksum mismatch: %x != %x ", hdr.Checksum, checksum)
	}

	cmdType := string(bytes.TrimRight(hdr.CMD[:], string(0)))
	msg, err := MakeEmptyMessage(cmdType)
	if err != nil {
		return nil, err
	}

	err = msg.Deserialization(buf)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func MakeEmptyMessage(cmdType string) (Message, error) {
	switch cmdType {
	case common.VERSION_TYPE:
		return &Version{}, nil
	case common.VERACK_TYPE:
		return &VerACK{}, nil
	case common.ADDR_TYPE:
		return &Addr{}, nil
	case common.GetADDR_TYPE:
		return &AddrReq{}, nil
	case common.PING_TYPE:
		return &Ping{}, nil
	case common.PONG_TYPE:
		return &Pong{}, nil
	case common.DISCONNECT_TYPE:
		return &Disconnected{}, nil
	case common.TEST_NETWORK:
		return &TestNetwork{}, nil
	case common.TX_TYPE:
		return &TxMsg{}, nil
	case common.SYNC_TYPE:
		return &SyncMsg{}, nil
	case common.CONS_TYPE:
		return &ConsMsg{}, nil
	default:
		return nil, errors.New("unsupported cmd type:" + cmdType)
	}

}

//caculate checksum value
func CheckSum(p []byte) [common.CHECKSUM_LEN]byte {
	var checksum [common.CHECKSUM_LEN]byte
	t := sha256.Sum256(p)
	s := sha256.Sum256(t[:])

	copy(checksum[:], s[:])

	return checksum
}
