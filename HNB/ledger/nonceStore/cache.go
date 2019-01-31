package nonceStore

import (
	"HNB/logging"
	"github.com/bluele/gcache"
)

var BlockLog logging.LogModule

const (
	LOGTABLE_BLOCK string = "block"
)

type nonceCache struct {
	cache gcache.Cache
}

func NewNonceCache() *nonceCache {
	bc := &nonceCache{}
	bc.cache = gcache.New(50).Build()
	BlockLog.Infof(LOGTABLE_BLOCK, "create block cache size:%v", 50)
	return bc
}

func (bc *nonceCache) SetNonce(address []byte, nonce uint64) error {
	BlockLog.Debugf(LOGTABLE_BLOCK, "set nonce:%v", nonce)
	return bc.cache.Set(string(address), nonce)
}

func (bc *nonceCache) GetNonce(address []byte) (uint64, error) {
	nonce, err := bc.cache.Get(string(address))
	if err != nil && err != gcache.KeyNotFoundError {
		return 0, nil
	}

	if nonce == nil {
		return 0, nil
	}

	BlockLog.Debugf(LOGTABLE_BLOCK, "get nonce:%v", nonce)

	return nonce.(uint64), nil
}
