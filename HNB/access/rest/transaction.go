package rest

import (
	"HNB/appMgr"
	appComm "HNB/appMgr/common"
	"HNB/common"
	"HNB/config"
	"HNB/contract/hgs"
	"HNB/contract/hnb"
	"HNB/msp"
	"HNB/rlp"
	"HNB/txpool"
	"HNB/util"
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
)

func QueryBalanceMsg(params json.RawMessage) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(params))
	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, errors.New("no [")
	}

	if !dec.More() {
		return nil, errors.New("data not complete")
	}
	var chainID, addr string
	err := dec.Decode(&chainID)
	if err != nil {
		return nil, err
	}
	if !dec.More() {
		return nil, errors.New("data not complete")
	}
	err = dec.Decode(&addr)
	if err != nil {
		return nil, err
	}

	if chainID == appComm.HGS {
		qh := &hgs.QryHgsTx{}
		qh.TxType = hnb.BALANCE
		qh.PayLoad = util.HexToByte(addr)
		qhm, _ := json.Marshal(qh)

		msg, err := appMgr.Query(chainID, qhm)
		if err != nil {
			return nil, err
		} else {
			var bal int64
			if msg == nil {
				bal = 0
			} else {
				bal, _ = strconv.ParseInt(string(msg), 10, 64)
			}
			return bal, nil
		}
	} else if chainID == appComm.HNB {
		qh := &hnb.QryHnbTx{}
		qh.TxType = hnb.BALANCE
		qh.PayLoad = util.HexToByte(addr)
		qhm, _ := json.Marshal(qh)

		msg, err := appMgr.Query(chainID, qhm)
		if err != nil {
			return nil, err
		} else {
			var bal int64
			if msg == nil {
				bal = 0
			} else {
				bal, _ = strconv.ParseInt(string(msg), 10, 64)
			}
			return bal, nil
		}
	} else {
		return nil, errors.New("chainID not exist")
	}
	return nil, nil
}

type AccountLock struct {
	rw      sync.RWMutex
	accLock map[common.Address]*sync.RWMutex
}

var AccLockMgr AccountLock

func (alm *AccountLock) Lock(address common.Address) {

	alm.rw.Lock()
	defer alm.rw.Unlock()

	if alm.accLock == nil {
		alm.accLock = make(map[common.Address]*sync.RWMutex)
	}

	lock, ok := alm.accLock[address]
	if !ok {
		lock = &sync.RWMutex{}
		alm.accLock[address] = lock
	}

	lock.Lock()
}

func (alm *AccountLock) UnLock(address common.Address) {

	alm.rw.RLock()
	defer alm.rw.RUnlock()

	lock, _ := alm.accLock[address]
	lock.Unlock()
}

func SendTxMsg(params json.RawMessage) (interface{}, error) {
	//from
	//value1
	//contractName
	//to
	//value2
	//contractName

	//nonceValue
	//  V/R/S

	dec := json.NewDecoder(bytes.NewReader(params))
	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, errors.New("no [")
	}

	if !dec.More() {
		return nil, errors.New("data not complete")
	}

	var rawTransaction string
	err := dec.Decode(&rawTransaction)
	if err != nil {
		return nil, err
	}

	msgTx := &common.Transaction{}
	err = rlp.DecodeBytes(util.FromHex(rawTransaction), msgTx)
	if err != nil {
		return nil, err
	}

	msgTx.Txid = common.Hash{}

	//测试使用
	if config.Config.RunMode != "dev" {
		address := msp.AccountPubkeyToAddress()
		if msgTx.NonceValue == 0 {
			AccLockMgr.Lock(address)
			defer AccLockMgr.UnLock(address)

			nonce := txpool.GetPendingNonce(address)
			msgTx.NonceValue = nonce
		}
	}

	//msgTx.From = address
	//signer := msp.GetSigner()
	//msgTx.Txid = signer.Hash(&msgTx)
	//msgTxWithSign, err := msp.SignTx(&msgTx, signer)
	//if err != nil {
	//	msg := fmt.Sprintf("sign err:%v", err.Error())
	//	retMsg := FormatQueryResResult("0001", msg, nil)
	//	encoder.Encode(retMsg)
	//	return
	//}
	signer := msp.GetSigner()
	msgTx.Txid = signer.Hash(msgTx)
	mar, _ := json.Marshal(msgTx)
	err = txpool.RecvTx(mar)
	if err != nil {
		return nil, err
	}
	return util.ByteToHex(msgTx.Txid.GetBytes()), nil

}
