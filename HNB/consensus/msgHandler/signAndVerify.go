package msgHandler

import (
	"encoding/json"
	"fmt"
	cmn "HNB/consensus/algorand/common"
	"HNB/msp"
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
	//publicKey := message.PublicKey
	//pk ,err := h.coor.PubKeyDecode(string(publicKey))
	sign := message.Signature
	//tdmLogger.Infof("verify sign %s", sign)

	//message.PublicKey = nil
	message.Signature = nil
	defer func() {
		//message.PublicKey = publicKey
		message.Signature = sign
	}()
	c, err := json.Marshal(message)
	if err != nil {
		return err
	}

	//msp.GetBccspKeyFromPubKey()

	ok, err := msp.Verify(nil, sign, c)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("verify faild")
	}
	return nil
}
