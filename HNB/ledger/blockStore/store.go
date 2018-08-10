package blockStore

import (
	"HNB/ledger/blockStore/common"
	dbComm "HNB/db/common"
	"encoding/binary"
	"encoding/json"
)

type BlockStore struct {
	cache *blockCache
	db  dbComm.KVStore
}

func NewBlockStore(db dbComm.KVStore) common.BlockStore{
	bc := &BlockStore{db:db}
	bc.cache = NewBlockCache()
	return bc
}

func(bc *BlockStore) WriteBlock(block *common.Block) error{
	blk, err := bc.cache.GetBlock(block.BlockNum)
	if err != nil{
		return err
	}
	if blk == nil{
		bc.cache.WriteBlock(block)
	}
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, block.BlockNum)

	blkM,_ := json.Marshal(block)

	return bc.db.Put(b, blkM)
}

func(bc *BlockStore) RollbackBlock(blkNum uint64) error{
	//优先检查缓存调用
	blk, err := bc.cache.GetBlock(blkNum)
	if err != nil{
		return err
	}
	if blk != nil{
		bc.cache.RollbackBlock(blkNum)
	}

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, blkNum)

	bc.db.Delete(b)
	return nil
}

func(bc *BlockStore) GetBlock(blkNum uint64) (*common.Block, error){
	//优先检查缓存调用
	blk, err := bc.cache.GetBlock(blkNum)
	if err != nil{
		return nil, err
	}
	if blk != nil{
		return nil, nil
	}

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, blkNum)

	blk1, err := bc.db.Get(b)

	if err != nil{
		return nil, err
	}

	blk2 := common.Block{}
	json.Unmarshal(blk1, &blk2)

	return &blk2, nil
}


