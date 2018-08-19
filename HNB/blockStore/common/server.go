package common


type BlockStore interface {
	WriteBlock(block *Block) error
	RollbackBlock(blkNum uint64) error
	GetBlock(blkNum uint64) (*Block, error)
}
