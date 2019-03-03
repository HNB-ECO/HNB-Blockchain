package appMgr

import (
	appComm "github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/contract/hgs"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/contract/hnb"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	bsComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/blockStore/common"
	nnComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/nonceStore/common"
	ssComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/stateStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
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
		if bytes.Compare(iter.Key(), []byte("contract#"+appComm.HGS)) == 0 {
			hgsInstalled = true
		}
	}

	hgsHandler, _ := hgs.GetContractHandler()

	if hgsInstalled == false {
		//安装智能合约
		apiInf := GetContractApi(appComm.HGS)
		err := hgs.InstallContract(apiInf)
		if err != nil {
			return err
		}
		s := &ssComm.StateSet{}
		s.SI = apiInf.GetAllState()
		ledger.SetContractState(s)
		db.Put([]byte("contract#"+appComm.HGS), []byte("active"))
	}

	am.setHandler(appComm.HGS, hgsHandler)
	//智能合约启动初始化
	err := initApp(appComm.HGS)
	if err != nil {
		appMgrLog.Info(LOGTABLE_APPMGR, "init hgs app err:"+err.Error())
	} else {
		appMgrLog.Info(LOGTABLE_APPMGR, "init hgs app")
	}

	hnbHandler, _ := hnb.GetContractHandler()

	if hnbInstalled == false {
		//安装智能合约
		apiInf := GetContractApi(appComm.HNB)
		err := hnb.InstallContract(apiInf)
		if err != nil {
			return err
		}
		s := &ssComm.StateSet{}
		s.SI = apiInf.GetAllState()
		ledger.SetContractState(s)
		db.Put([]byte("contract#"+appComm.HNB), []byte("active"))
	}

	am.setHandler(appComm.HNB, hnbHandler)
	//智能合约启动初始化
	err = initApp(appComm.HNB)
	if err != nil {
		appMgrLog.Info(LOGTABLE_APPMGR, "init hnb app err:"+err.Error())
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

func CheckTx(tx *common.Transaction) error {
	handler, err := am.getHandler(tx.ContractName)
	if err != nil {
		return err
	}

	if handler == nil {
		return errors.New("app not exist")
	}
	apiInf := GetContractApi(tx.ContractName)
	apiInf.SetArgs(tx.Payload)
	apiInf.SetFrom(tx.FromAddress())

	return handler.CheckTx(apiInf)
}

func BlockProcess(blk *bsComm.Block) error {
	if blk == nil {
		return errors.New("blk = nil")
	}

	apiHandlers := make(map[string]*contractApi)
	n := &nnComm.NonceSet{}
	s := &ssComm.StateSet{}

	for _, tx := range blk.Txs {
		handler, err := am.getHandler(tx.ContractName)
		if err != nil {
			return err
		}

		h, ok := apiHandlers[tx.ContractName]
		if !ok {
			h = GetContractApi(tx.ContractName)
			apiHandlers[tx.ContractName] = h
		}

		h.SetArgs(tx.Payload)
		h.SetFrom(tx.FromAddress())

		snapshot := h.GetSnapshot()
		err = handler.Invoke(h)
		if err != nil {
			//TODO 添加事件通知 将错误事件记录到异常表
			ledger.SetWrongIndex(tx.Txid[:],err.Error())
			//return err
			appMgrLog.Info(LOGTABLE_APPMGR, "hnb err:"+err.Error())
			h.SnapshotRestore(snapshot)
		}

		ni := &nnComm.NonceItem{}
		ni.Nonce = tx.Nonce() + 1
		ni.Key = tx.FromAddress()
		n.NI = append(n.NI, ni)
	}

	stateChange, _ := json.Marshal([]interface{}{s.SI, n.NI})

	for _, v := range apiHandlers {
		s.SI = append(s.SI, v.GetAllState()...)
	}

	blk.Header.StateHash, _ = msp.Hash256(stateChange)

	return ledger.WriteLedger(blk, s, n)
}

func UnfreezeToken(epochNo uint64) error {
	handler, err := am.getHandler(appComm.HNB)
	if err != nil {
		return err
	}

	if handler == nil {
		return errors.New("chainID not exist")
	}

	h := GetContractApi(appComm.HNB)
	opochNoS := strconv.FormatUint(epochNo, 10)

	hb := &hnb.HnbTx{}
	hb.TxType = hnb.UNFREEZE_TOKEN
	hb.PayLoad = []byte(opochNoS)

	payload, _ := json.Marshal(hb)
	h.SetArgs(payload)
	err = handler.Invoke(h)
	if err != nil {
		return err
	}

	ss := ssComm.StateSet{}
	ss.SI = h.GetAllState()
	err = ledger.SetContractState(&ss)
	if err != nil {
		return err
	}
	return nil
}

type QryHnbTx struct {
	//1  balance   2  ....
	TxType  uint8  `json:"txType"`
	PayLoad []byte `json:"payLoad"`
}

func GetVoteSum(epochNo uint64) (int64, error) {
	h, _ := am.getHandler(appComm.HNB)
	apiInf := GetContractApi(appComm.HNB)
	opochNoS := strconv.FormatUint(epochNo, 10)
	qht := &QryHnbTx{}
	qht.TxType = hnb.VOTESUM
	qht.PayLoad = []byte(opochNoS)

	q, _ := json.Marshal(qht)
	apiInf.SetArgs(q)

	r, err := h.Query(apiInf)
	if err != nil {
		return 0, err
	}

	s, err := strconv.ParseInt(string(r), 10, 64)
	if err != nil {
		return 0, err
	}

	return s, nil
}

func GetVotePeerIDOrderToken(epochNo uint64) ([][]byte, error) {
	h, _ := am.getHandler(appComm.HNB)
	apiInf := GetContractApi(appComm.HNB)

	opochNoS := strconv.FormatUint(epochNo, 10)
	qht := &QryHnbTx{}
	qht.TxType = hnb.TOKENORDER
	qht.PayLoad = []byte(opochNoS)
	q, _ := json.Marshal(qht)
	apiInf.SetArgs(q)

	r, err := h.Query(apiInf)
	if err != nil {
		return nil, err
	}

	pis := hnb.PeersIDSet{}
	err = json.Unmarshal(r, &pis)
	if err != nil {
		return nil, err
	}

	return pis.PeerIDs, nil
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
