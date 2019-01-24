package blockStore

import (
	"encoding/binary"
	"encoding/json"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/blockStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
)

type BlockStore struct {
	cache *blockCache
	db    dbComm.KVStore
}

func NewBlockStore(db dbComm.KVStore) common.BlockStore {
	BlockLog = logging.GetLogIns()
	bc := &BlockStore{db: db}
	bc.cache = NewBlockCache()
	BlockLog.Info(LOGTABLE_BLOCK, "create block store")
	return bc
}

func (bc *BlockStore) WriteBlock(block *common.Block) error {
	BlockLog.Infof(LOGTABLE_BLOCK, "write block num:%d", block.Header.BlockNum)

	bc.cache.WriteBlock(block)

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, block.Header.BlockNum)

	blkM, _ := json.Marshal(block)

	BlockLog.Debugf(LOGTABLE_BLOCK, "write db block num:%d", block.Header.BlockNum)

	return bc.db.Put(b, blkM)
}

func (bc *BlockStore) RollbackBlock(blkNum uint64) error {
	BlockLog.Debugf(LOGTABLE_BLOCK, "del block num:%d", blkNum)
	//优先检查缓存调用
	blk, err := bc.cache.GetBlock(blkNum)
	if err != nil {
		return err
	}
	if blk != nil {
		bc.cache.RollbackBlock(blkNum)
	}

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, blkNum)

	bc.db.Delete(b)
	return nil
}
func (bc *BlockStore) GetBlockHash(blkNum uint64) ([]byte, error) {
	blk, err := bc.GetBlock(blkNum)
	if err != nil {
		return nil, err
	}
	//blkMarshal, err := json.Marshal(blk.Header)
	//if err != nil{
	//	return nil, err
	//}
	////TODO 计算Hash
	//panic("calc hash")

	//return blkMarshal, err
	return bc.CalcBlockHash(blk)
}
func (bc *BlockStore) GetBlock(blkNum uint64) (*common.Block, error) {
	//优先检查缓存调用
	blk, err := bc.cache.GetBlock(blkNum)
	if err != nil {
		return nil, err
	}
	if blk != nil {
		BlockLog.Debugf(LOGTABLE_BLOCK, "read block cache num:%d", blkNum)
		return blk, nil
	}

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, blkNum)

	BlockLog.Debugf(LOGTABLE_BLOCK, "read db block num:%d", blkNum)
	blk1, err := bc.db.Get(b)

	if err != nil {
		return nil, err
	}

	blk2 := common.Block{}
	err = json.Unmarshal(blk1, &blk2)
	return &blk2, err
}

func (bc *BlockStore) CalcBlockHash(blk *common.Block) ([]byte, error) {
	blkMarshal, err := json.Marshal(blk.Header)
	if err != nil {
		return nil, err
	}
	return msp.DoubleHash256(blkMarshal)
}
