package dbft

import (
	"encoding/json"
	"fmt"
	dpos "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/dbft/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
)

func (dbftMgr *DBFTManager) Sign(message *dpos.DPoSMessage) (*dpos.DPoSMessage, error) {
	if message == nil {
		return nil, fmt.Errorf("sign msg is nil")
	}
	c, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	sign, err := msp.Sign(c)
	if err != nil {
		return nil, err
	}
	message.Signature = sign

	return message, nil
}

func (dbftMgr *DBFTManager) Verify(message *dpos.DPoSMessage, pubKeyID []byte) error {

	if message == nil {
		return fmt.Errorf("verify msg is nil")
	}
	sign := message.Signature
	message.Signature = nil
	defer func() {
		message.Signature = sign
	}()
	c, err := json.Marshal(message)
	if err != nil {
		return err
	}
	keyString := msp.PeerIDToString(pubKeyID)
	ok, err := msp.Verify(msp.StringToBccspKey(keyString), sign, c)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("verify faild")
	}
	return nil
}
