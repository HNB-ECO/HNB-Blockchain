package msgHandler

import (
	cmn "HNB/consensus/algorand/common"
	"HNB/msp"
	"encoding/json"
	"fmt"
)

func (h *TDMMsgHandler) Sign(message *cmn.TDMMessage) (*cmn.TDMMessage, error) {
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

func (h *TDMMsgHandler) Verify(message *cmn.TDMMessage, pubKeyID []byte) error {

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
