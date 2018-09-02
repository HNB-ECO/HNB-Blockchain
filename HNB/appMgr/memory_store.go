package appMgr

import (
	"HNB/ledger/stateStore/common"
)

type StateCache struct {
	memory map[string]*common.StateItem
}

func NewStateCache() *StateCache {
	return &StateCache{
		memory: make(map[string]*common.StateItem),
	}
}

func (db *StateCache) Put(chainID string, key []byte, value []byte) {
	db.memory[string(append([]byte(chainID), key...))] = &common.StateItem{
		Key:   key,
		Value: value,
		State: common.Changed,
		ChainID:chainID,
	}
}

func (db *StateCache) Get(chainID string, key []byte) *common.StateItem {
	if entry, ok := db.memory[string(append([]byte(chainID), key...))]; ok {
		return entry
	}
	return nil
}

func (db *StateCache) Delete(chainID string, key []byte) {
	if v, ok := db.memory[string(append([]byte(chainID), key...))]; ok {
		v.State = common.Deleted
	} else {
		db.memory[string(append([]byte(chainID), key...))] = &common.StateItem{
			Key:   key,
			State: common.Deleted,
			ChainID:chainID,
		}
	}

}

func (db *StateCache) Find() []*common.StateItem {
	var memory []*common.StateItem
	for _, v := range db.memory {
		memory = append(memory, v)
	}
	return memory
}

func (db *StateCache) GetChangeSet() map[string]*common.StateItem {
	m := make(map[string]*common.StateItem)
	for k, v := range db.memory {
		if v.State != common.None {
			m[k] = v
		}
	}
	return m
}
