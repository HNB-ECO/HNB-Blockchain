package rest

import (
	"HNB/appMgr"
	"HNB/common"
	"HNB/msp"
	"HNB/txpool"
	"HNB/util"
	"encoding/json"
	"fmt"
	"github.com/gocraft/web"
	"io/ioutil"
	"net/http"
	"sync"
)

func (*serverREST) QueryMsg(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)

	addr := req.PathParams["addr"]

	msg, err := appMgr.Query("hgs", util.HexToByte(addr))
	var retMsg interface{}
	if err != nil {
		retMsg = FormatQueryResResult("0001", err.Error(), string(msg))
	} else {
		retMsg = FormatQueryResResult("0000", "", string(msg))
	}

	encoder.Encode(retMsg)
}

type AccountLock struct {
	rw      sync.RWMutex
	accLock map[common.Address]*sync.RWMutex
}

var AccLockMgr AccountLock

func (alm AccountLock) Lock(address common.Address) {

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

func (alm AccountLock) UnLock(address common.Address) {

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

	address := msp.AccountPubkeyToAddress()
	if msgTx.NonceValue == 0 {
		AccLockMgr.Lock(address)
		defer AccLockMgr.UnLock(address)

		msgTx.NonceValue = txpool.GetPendingNonce(address)
	}

	msgTx.From = address

	signer := msp.GetSigner()
	msgTx.Txid = signer.Hash(&msgTx)

	msgTxWithSign, err := msp.SignTx(&msgTx, signer)
	if err != nil {
		msg := fmt.Sprintf("sign err:%v", err.Error())
		retMsg := FormatQueryResResult("0001", msg, nil)
		encoder.Encode(retMsg)
		return
	}

	mar, _ := json.Marshal(msgTxWithSign)
	txpool.RecvTx(mar)

	retMsg := FormatInvokeResResult("0000", "", msgTx.Txid)
	encoder.Encode(retMsg)
}
