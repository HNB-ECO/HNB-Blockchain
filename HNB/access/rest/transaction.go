package rest

import (
	"HNB/appMgr"
	"HNB/common"
	"HNB/contract/hgs"
	"HNB/contract/hnb"
	"HNB/msp"
	"HNB/txpool"
	"HNB/util"
	"encoding/json"
	"fmt"
	"github.com/gocraft/web"
	"io/ioutil"
	"net/http"
	"sync"
	"strconv"
)

//type qryHnbTx struct {
//	//1  balance   2  ....
//	TxType     uint8  `json:"txType"`
//	PayLoad    []byte `json:"payLoad"`
//}

func (*serverREST) QueryBalanceMsg(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)

	addr := req.PathParams["addr"]
	chainID := req.PathParams["chainID"]

	if chainID == txpool.HGS {
		qh := &hgs.QryHgsTx{}
		qh.TxType = hnb.BALANCE
		qh.PayLoad = util.HexToByte(addr)
		qhm, _ := json.Marshal(qh)

		var retMsg interface{}
		msg, err := appMgr.Query(chainID, qhm)
		if err != nil {
			retMsg = FormatQueryResResult("0001", err.Error(), string(msg))
		} else {
			if msg == nil{
				retMsg = FormatQueryResResult("0000", "", 0)
			}else{

				bal,_ := strconv.ParseInt(string(msg), 10, 64)
				retMsg = FormatQueryResResult("0000", "", bal)
			}
		}
		encoder.Encode(retMsg)
	} else if chainID == txpool.HNB {
		qh := &hnb.QryHnbTx{}
		qh.TxType = hnb.BALANCE
		qh.PayLoad = util.HexToByte(addr)
		qhm, _ := json.Marshal(qh)

		var retMsg interface{}
		msg, err := appMgr.Query(chainID, qhm)
		if err != nil {
			retMsg = FormatQueryResResult("0001", err.Error(), string(msg))
		} else {
			if msg == nil{
				retMsg = FormatQueryResResult("0000", "", 0)
			}else{

				bal,_ := strconv.ParseInt(string(msg), 10, 64)
				retMsg = FormatQueryResResult("0000", "", bal)
			}
		}

		encoder.Encode(retMsg)
	} else {
		retMsg := FormatQueryResResult("0001", "chainid invalid", nil)
		encoder.Encode(retMsg)
	}
	return
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

func (*serverREST) SendTxMsg(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	reqBody, _ := ioutil.ReadAll(req.Body)
	msgTx := common.Transaction{}
	err := json.Unmarshal(reqBody, &msgTx)
	if err != nil {
		msg := fmt.Sprintf("reqBody err:%v", err.Error())
		retMsg := FormatQueryResResult("0001", msg, nil)
		encoder.Encode(retMsg)
		return
	}

	msgTx.Txid = common.Hash{}
	address := msp.AccountPubkeyToAddress()
	if msgTx.NonceValue == 0 {
		AccLockMgr.Lock(address)
		defer AccLockMgr.UnLock(address)

		nonce := txpool.GetPendingNonce(address)
		//if nonce == 0{
		//	nonce = 1
		//}
		msgTx.NonceValue = nonce
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
	msgTx.Txid = signer.Hash(&msgTx)
	mar, _ := json.Marshal(msgTx)
	err = txpool.RecvTx(mar)
	if err != nil {
		retMsg := FormatInvokeResResult("0001", err.Error(), common.Hash{})
		encoder.Encode(retMsg)
	} else {
		retMsg := FormatInvokeResResult("0000", "", msgTx.Txid)
		encoder.Encode(retMsg)
	}

}

