package msgHandler

import (
	"encoding/json"
	"fmt"
	cmn "HNB/consensus/algorand/common"
)

func (h *TDMMsgHandler) Sign(message *cmn.TDMMessage) (*cmn.TDMMessage, error) {
	if message == nil {
		return nil, fmt.Errorf("sign msg is nil")
	}
	c, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	//TODO sign
	sign, err := h.coor.Sign(c)
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

	//TODO verify
	pk, err := h.Network.GetPK(peerId.Name + "_" + peerId.OrgId)
	if err != nil {
		return err
	}
	//tdmLogger.Infof("verify pk %v msg %v sign %v", pk, c, sign)
	ok, err := h.coor.Verify(pk, c, sign)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("verify faild")
	}
	return nil
}
