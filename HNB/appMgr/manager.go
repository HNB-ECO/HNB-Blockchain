package appMgr

import (
	appComm "HNB/appMgr/common"
	"HNB/contract/hgs"
	"HNB/contract/hnb"
	dbComm "HNB/db/common"
	"HNB/ledger"
	"HNB/ledger/blockStore/common"
	nnComm "HNB/ledger/nonceStore/common"
	ssComm "HNB/ledger/stateStore/common"
	"HNB/logging"
	"HNB/msp"
	"bytes"
	"encoding/json"
	"errors"
	"sync"
	"HNB/txpool"
)

type appManager struct {
	scs map[string]appComm.ContractInf
	sync.RWMutex
	db dbComm.KVStore
}

const (
	LOGTABLE_APPMGR = "appmgr"
)

var am *appManager
var appMgrLog logging.LogModule

func initApp(chainID string) error {
	h, _ := am.getHandler(chainID)
	err := h.Init()
	if err != nil {
		return err
	}
	return nil
}

func InitAppMgr(db dbComm.KVStore) error {
	//加载合约
	appMgrLog = logging.GetLogIns()
	appMgrLog.Info(LOGTABLE_APPMGR, "ready start app manager")

	am = &appManager{}
	am.scs = make(map[string]appComm.ContractInf)
	am.db = db

	hgsInstalled := false
	hnbInstalled := false

	iter := db.NewIterator([]byte("contract#"))

	for iter.Next() {
		appMgrLog.Infof(LOGTABLE_APPMGR, "installed contract %v", string(iter.Key()))
		if bytes.Compare(iter.Key(), []byte("contract#" + txpool.HGS)) == 0 {
			hgsInstalled = true
		}
	}

	hgsHandler, _ := hgs.GetContractHandler()

	if hgsInstalled == false {
		//安装智能合约
		apiInf := GetContractApi(txpool.HGS)
		err := hgs.InstallContract(apiInf)
		if err != nil {
			return err
		}
		s := &ssComm.StateSet{}
		s.SI = apiInf.GetAllState()
		ledger.SetContractState(s)
		db.Put([]byte("contract#" + txpool.HGS), []byte("active"))
	}

	am.setHandler(txpool.HGS, hgsHandler)
	//智能合约启动初始化
	err := initApp(txpool.HGS)
	if err != nil {
		appMgrLog.Info(LOGTABLE_APPMGR, "init hgs app err:"+err.Error())
	} else {
		appMgrLog.Info(LOGTABLE_APPMGR, "init hgs app")
	}


	hnbHandler, _ := hnb.GetContractHandler()
	
	if hnbInstalled == false{
		//安装智能合约
		apiInf := GetContractApi(txpool.HNB)
		err := hnb.InstallContract(apiInf)
		if err != nil {
			return err
		}
		s := &ssComm.StateSet{}
		s.SI = apiInf.GetAllState()
		ledger.SetContractState(s)
		db.Put([]byte("contract#" + txpool.HNB), []byte("active"))
	}

	am.setHandler(txpool.HNB, hnbHandler)
	//智能合约启动初始化
	err = initApp(txpool.HNB)
	if err != nil {
		appMgrLog.Info(LOGTABLE_APPMGR, "init hnb app err:" + err.Error())
	} else {
		appMgrLog.Info(LOGTABLE_APPMGR, "init hnb app")
	}

	return nil
}


//记录合约地址

func Query(chainID string, args []byte) ([]byte, error) {
	h, _ := am.getHandler(chainID)
	apiInf := GetContractApi(chainID)
	apiInf.SetArgs(args)

	return h.Query(apiInf)
}

func BlockProcess(blk *common.Block) error {
	if blk == nil {
		return errors.New("blk = nil")
	}

	apiHandlers := make(map[string]*contractApi)
	n := &nnComm.NonceSet{}
	s := &ssComm.StateSet{}

	for _, tx := range blk.Txs {
		handler, err := am.getHandler(tx.Type)
		if err != nil {
			return err
		}

		h, ok := apiHandlers[tx.Type]
		if !ok{
			h = GetContractApi(tx.Type)
			apiHandlers[tx.Type] = h
		}

		h.SetArgs(tx.Payload)
		h.SetFrom(tx.FromAddress())

		snapshot := h.GetSnapshot()
		err = handler.Invoke(h)
		if err != nil {
			//TODO 添加事件通知
			//return err
			appMgrLog.Info(LOGTABLE_APPMGR, "hgs err:"+err.Error())
			h.SnapshotRestore(snapshot)
		}

		ni := &nnComm.NonceItem{}
		ni.Nonce = tx.Nonce() + 1
		ni.Key = tx.FromAddress()
		n.NI = append(n.NI, ni)
	}

	stateChange, _ := json.Marshal([]interface{}{s.SI, n.NI})

	for _,v := range apiHandlers{
		s.SI = append(s.SI, v.GetAllState()...)
	}


	blk.StateHash, _ = msp.Hash256(stateChange)

	return ledger.WriteLedger(blk, s, n)
}

func BlockRollBack(blkNum uint64) error {
	return nil
}

func (am *appManager) setHandler(chainID string, inf appComm.ContractInf) {
	am.Lock()
	am.scs[chainID] = inf
	am.Unlock()
}

func (am *appManager) getHandler(chainID string) (appComm.ContractInf, error) {
	am.RLock()
	defer am.RUnlock()

	handler, ok := am.scs[chainID]
	if !ok {
		return nil, errors.New("chainID not exist")
	}

	return handler, nil
}

