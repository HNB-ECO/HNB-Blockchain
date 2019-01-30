package stateStore

import (
	"bytes"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/stateStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
)

type stateStore struct {
	cache *StateCache
	db    dbComm.KVStore
}

func NewStateStore(db dbComm.KVStore) common.StateStore {
	StateLog = logging.GetLogIns()
	ss := &stateStore{db: db}
	ss.cache = NewStateCache()

	return ss
}

func (ss *stateStore) GetState(chainID string, key []byte) ([]byte, error) {
	var err error
	key1 := BytesCombine([]byte(chainID+"#"), key)
	v := ss.cache.GetState(key1)
	if v == nil {
		v, err = ss.db.Get(key1)
	}
	StateLog.Debugf(LOGTABLE_STATE, "get state db key:%v value:%v", string(key1), string(v))
	return v, err
}

func (ss *stateStore) SetState(chainID string, key []byte, state []byte) error {
	key1 := BytesCombine([]byte(chainID+"#"), key)
	ss.cache.SetState(key1, state)
	StateLog.Debugf(LOGTABLE_STATE, "set state db key:%v value:%v", string(key1), string(state))
	return ss.db.Put(key1, state)
}

func (ss *stateStore) DeleteState(chainID string, key []byte) error {
	key1 := BytesCombine([]byte(chainID+"#"), key)
	ss.cache.DeleteState(key1)
	StateLog.Debugf(LOGTABLE_STATE, "remove state db key:%v", string(key1))
	return ss.db.Delete(key1)
}

func BytesCombine(pBytes ...[]byte) []byte {
	return bytes.Join(pBytes, []byte(""))
}
