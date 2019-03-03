package nonceStore

import (
	cn "github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/nonceStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"encoding/binary"
)

type NonceStore struct {
	cache *nonceCache
	db    dbComm.KVStore
}

func NewNonceStore(db dbComm.KVStore) common.NonceStore {
	BlockLog = logging.GetLogIns()
	bc := &NonceStore{db: db}
	bc.cache = NewNonceCache()
	BlockLog.Info(LOGTABLE_BLOCK, "create block store")
	return bc
}

func (bc *NonceStore) SetNonce(address cn.Address, nonce uint64) error {
	BlockLog.Infof(LOGTABLE_BLOCK, "set nonce:%d", nonce)

	key := append([]byte("nonce"), address[:]...)

	bc.cache.SetNonce(key, nonce)

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, nonce)

	return bc.db.Put(key, b)
}
func (bc *NonceStore) GetNonce(address cn.Address) (uint64, error) {

	key := append([]byte("nonce"), address[:]...)

	nonce, err := bc.cache.GetNonce(key)
	if err != nil {
		return 0, err
	}

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, nonce)

	nonceByte, err := bc.db.Get(key)

	if err != nil {
		return 0, err
	}

	if nonceByte == nil {
		return 0, nil
	}

	return binary.BigEndian.Uint64(nonceByte), nil
}
