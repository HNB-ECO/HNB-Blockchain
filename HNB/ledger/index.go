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
			is := indexStore{block.Header.BlockNum, uint32(i)}
			isMar, _ := json.Marshal(&is)
			key := append([]byte(INDEX), v.Txid[:]...)
			//fmt.Printf("index key:%v\n", key)
			err := lh.dbHandler.Put(key, isMar)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func FindHashIndex(txid []byte) (*indexStore, error) {
	is := indexStore{}
	key := append([]byte(INDEX), txid[:]...)
	//fmt.Printf("query key:%v\n", key)
	isMar, err := lh.dbHandler.Get(key)
	if err != nil {
		return nil, err
	}
	if isMar == nil {
		return nil, nil
	}
	err = json.Unmarshal(isMar, &is)
	if err != nil {
		return nil, err
	}
	return &is, nil
}
