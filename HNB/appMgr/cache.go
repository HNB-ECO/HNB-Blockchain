
pckage appMgr

import (
	"HNB/logging"
	"github.com/bluele/gcache"
)

var StateLog logging.LogModule

const (
	LOGTABLE_STATE string = "nonceState"
)

const (
	STATE_CACHE_SIZE = 100000
)

type NonceCache struct {
	cache gcache.Cache
}

func NewCache() *NonceCache {
	sc := gcache.New(STATE_CACHE_SIZE).Build()
	StateLog.Infof(LOGTABLE_STATE, "create state cache size:%v", STATE_CACHE_SIZE)
	return &NonceCache{
		cache: sc,
	}
}

func (this *NonceCache) GetState(key []byte) []byte {

	state, ok := this.cache.Get(string(key))
	if ok != nil {
		return nil
	}
	StateLog.Debugf(LOGTABLE_STATE, "get state cache key:%v value:%v", string(key), string(state.([]byte)))
	return state.([]byte)
}

func (this *NonceCache) SetState(key []byte, state []byte) {
	StateLog.Debugf(LOGTABLE_STATE, "set state cache key:%v value:%v", string(key), string(state))
	this.cache.Set(string(key), state)
}

func (this *NonceCache) DeleteState(key []byte) {
	StateLog.Debugf(LOGTABLE_STATE, "remove state cache key:%v", string(key))
	this.cache.Remove(string(key))
}
ackage appMgr

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
	"HNB/txpool"
	"bytes"
	"encoding/json"
	"errors"
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
