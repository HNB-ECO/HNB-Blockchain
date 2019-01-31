package nonceStore

import (
	dbComm "HNB/db/common"
	"HNB/ledger/wrongStore/common"
	"HNB/logging"
)

var BlockLog logging.LogModule

type WrongStore struct {
	db    dbComm.KVStore
}

const (
	LOGTABLE_WRONG string = "wrongIndex"
	WRONG = "wrong"
)

func NewWrongStore(db dbComm.KVStore) common.WrongIndexStore {
	BlockLog = logging.GetLogIns()
	bc := &WrongStore{db: db}
	BlockLog.Info(LOGTABLE_WRONG, "create wrong index store")
	return bc
}


func (ws *WrongStore)GetWrongIndex(txid []byte) (string, error){
	key := append([]byte(WRONG), txid[:]...)
	reason, err := ws.db.Get(key)
	if err != nil {
		return "", err
	}

	return string(reason), nil
}

func (ws *WrongStore)SetWrongIndex(txid []byte, reason string) error{
	key := append([]byte(WRONG), txid[:]...)
	err := ws.db.Put(key, []byte(reason))
	return err
}


