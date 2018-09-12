package blockStore

import (
	"github.com/bluele/gcache"
	"HNB/ledger/blockStore/common"
	"HNB/logging"
)

var BlockLog logging.LogModule
const(
	LOGTABLE_BLOCK string = "block"
)


type blockCache struct {
	cache gcache.Cache
}

func NewBlockCache() *blockCache{
	bc := &blockCache{}
	bc.cache = gcache.New(50).Build()
	BlockLog.Infof(LOGTABLE_BLOCK, "create block cache size:%v", 50)
	return bc
}

//batch操作

func(bc *blockCache) WriteBlock(block *common.Block) error{
	BlockLog.Debugf(LOGTABLE_BLOCK, "write block cache blkNum:%v", block.BlockNum)
	return bc.cache.Set(block.BlockNum, block)
}

func(bc *blockCache) RollbackBlock(blkNum uint64) error{
	BlockLog.Debugf(LOGTABLE_BLOCK, "rollback block cache blkNum:%v", blkNum)
	bc.cache.Remove(blkNum)
	return nil
}

func(bc *blockCache) GetBlock(blkNum uint64) (*common.Block, error){
	blk, err := bc.cache.Get(blkNum)
	if err != nil && err != gcache.KeyNotFoundError{
		//BlockLog.Debugf(LOGTABLE_BLOCK, "blk cache err:%v", err.Error())
		return nil, nil
	}

	if blk == nil{
		return nil, nil
	}

	BlockLog.Debugf(LOGTABLE_BLOCK, "get block cache blkNum:%v", blkNum)

	return blk.(*common.Block), nil
}





