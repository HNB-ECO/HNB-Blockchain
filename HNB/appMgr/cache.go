package appMgr

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
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
