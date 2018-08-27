package blockStore

import (
	"github.com/bluele/gcache"
	"HNB/ledger/blockStore/common"
)

type blockCache struct {
	cache gcache.Cache
}

func NewBlockCache() *blockCache{
	bc := &blockCache{}
	bc.cache = gcache.New(50).Build()
	return bc
}

//batch操作

func(bc *blockCache) WriteBlock(block *common.Block) error{
	return bc.cache.Set(block.BlockNum, block)
}

func(bc *blockCache) RollbackBlock(blkNum uint64) error{
	bc.cache.Remove(blkNum)
	return nil
}

func(bc *blockCache) GetBlock(blkNum uint64) (*common.Block, error){
	blk, err := bc.cache.Get(blkNum)
	if err != nil{
		return nil, err
	}
	return blk.(*common.Block), nil
}





