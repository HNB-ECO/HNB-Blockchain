package appMgr

import(
	"HNB/ledger/blockStore/common"
	appComm "HNB/appMgr/common"
	"HNB/contract/hgs"
	"sync"
	"errors"
	"HNB/logging"
	"HNB/ledger"
	ssComm "HNB/ledger/stateStore/common"
	dbComm "HNB/db/common"
	"bytes"
)

type appManager struct {
	scs map[string] appComm.ContractInf
	sync.RWMutex
	db dbComm.KVStore
}

const(
	LOGTABLE_APPMGR = "appmgr"
)

var am *appManager
var appMgrLog logging.LogModule


func initApp(chainID string) error{
	h,_ := am.getHandler(chainID)
	err := h.Init()
	if err != nil{
		return err
	}
	return nil
}

func InitAppMgr(db dbComm.KVStore) error{
	//加载合约
	appMgrLog = logging.GetLogIns()
	appMgrLog.Info(LOGTABLE_APPMGR, "ready start app manager")

	am = &appManager{}
	am.scs = make(map[string] appComm.ContractInf)
	am.db = db


	hgsInstalled := false
	iter := db.NewIterator([]byte("contract#"))

	for iter.Next() {
		appMgrLog.Infof(LOGTABLE_APPMGR, "installed contract %v", string(iter.Key()))
		if bytes.Compare(iter.Key(), []byte("contract#" + "hgs")) == 0{
			hgsInstalled = true
		}
	}

	hgsHandler, _ := hgs.GetContractHandler()

	if hgsInstalled == false{
		//安装智能合约
		apiInf := GetContractApi("hgs")
		err := hgs.InstallContract(apiInf)
		if err != nil{
			return err
		}
		s := &ssComm.StateSet{}
		s.SI = apiInf.GetAllState()
		ledger.SetContractState(s)
		db.Put([]byte("contract#" + "hgs"), []byte("active"))
	}


	am.setHandler("hgs", hgsHandler)
	//智能合约启动初始化
	err := initApp("hgs")
	if err != nil{
		appMgrLog.Info(LOGTABLE_APPMGR, "init hgs app err:" + err.Error())
	}else{
		appMgrLog.Info(LOGTABLE_APPMGR, "init hgs app")
	}

	return nil
}

//记录合约地址



func Query(chainID string, args string) ([]byte, error){
	h,_ := am.getHandler(chainID)
	apiInf := GetContractApi(chainID)
	apiInf.SetStringArgs(args)
	return h.Query(apiInf)
}



func BlockProcess(blk *common.Block) error{
	if blk == nil{
		return errors.New("blk = nil")
	}

	apiHandlers := make(map[string] *contractApi)

	for _, tx := range blk.Txs{
		handler, err := am.getHandler(tx.Type)
		if err != nil{
			return err
		}

		h, ok := apiHandlers[tx.Type]
		if !ok{
			h = GetContractApi(tx.Type)
			apiHandlers[tx.Type] = h
		}

		h.SetStringArgs(tx.Payload)
		err = handler.Invoke(h)
		if err != nil{
			//TODO 添加事件通知
			//return err
			appMgrLog.Info(LOGTABLE_APPMGR, "hgs err:" + err.Error())
		}
	}

	//在事务中更新块信息和状态信息
	s := &ssComm.StateSet{}

	for _, v := range apiHandlers{
		s.SI = append(s.SI, v.GetAllState()...)
	}


	return ledger.WriteLedger(blk, s)
}

func BlockRollBack(blkNum uint64) error{
	return nil
}

func (am *appManager) setHandler(chainID string, inf appComm.ContractInf){
	am.Lock()
	am.scs[chainID] = inf
	am.Unlock()
}

func (am *appManager) getHandler(chainID string) (appComm.ContractInf, error){
	am.RLock()
	defer am.RUnlock()

	handler, ok := am.scs[chainID]
	if !ok{
		return nil, errors.New("chainID not exist")
	}

	return handler, nil
}




