package ledger

import (
	bsComm "HNB/ledger/blockStore/common"
	"encoding/json"
)

type indexStore struct {
	BlockNum uint64
	Offset   uint32
}

const (
	INDEX = "index"
)

func SetHashIndex(block *bsComm.Block) error {
	if block != nil {
		for i, v := range block.Txs {
			is := &indexStore{block.BlockNum, uint32(i)}
			isMar, _ := json.Marshal(is)
			key := append([]byte(INDEX), v.Txid[:]...)
			err := lh.dbHandler.Put(key, isMar)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func FindHashIndex(txid string) (*indexStore, error) {
	is := indexStore{}
	key := append([]byte(INDEX), txid[:]...)
	isMar, err := lh.dbHandler.Get(key)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(isMar, &is)
	if err != nil {
		return nil, err
	}
	return &is, nil
}
