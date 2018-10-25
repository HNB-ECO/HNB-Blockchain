package ledger

import (
	dbComm "HNB/db/common"
	"HNB/ledger/blockStore"
	bsComm "HNB/ledger/blockStore/common"
	"HNB/ledger/stateStore"
	ssComm "HNB/ledger/stateStore/common"
	"HNB/logging"
	"errors"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/merkle"
)

// 系统数据在本文件中涉及存储，合约数据都在stateStore里面存储

var LedgerLog logging.LogModule

const (
	LOGTABLE_LEDGER string = "ledger"
)

type ledgerHandler struct {
	dbHandler    dbComm.KVStore
	blockHandler bsComm.BlockStore
	stateHandler ssComm.StateStore
}

var lh *ledgerHandler

func InitLedger(db dbComm.KVStore) error {
	LedgerLog = logging.GetLogIns()
	LedgerLog.Info(LOGTABLE_LEDGER, "Init ledger")
	lh = &ledgerHandler{}
	lh.dbHandler = db
	lh.blockHandler = blockStore.NewBlockStore(db)
	lh.stateHandler = stateStore.NewStateStore(db)

	return nil
}

func SetContractState(state *ssComm.StateSet) error {
	lh.dbHandler.NewBatch()
	if state != nil && state.SI != nil {
		for _, v := range state.SI {
			if v.State == ssComm.Deleted {
				lh.stateHandler.DeleteState(v.ChainID, v.Key)
			} else {
				lh.stateHandler.SetState(v.ChainID, v.Key, v.Value)
			}
		}
	}
	lh.dbHandler.BatchCommit()
	return nil
}

func WriteLedger(block *bsComm.Block, state *ssComm.StateSet) error {
	h, err := GetBlockHeight()
	if err != nil {
		return nil
	}

	if block.BlockNum != h {
		return errors.New("height invalid")
	}

	lh.dbHandler.NewBatch()
	lh.blockHandler.WriteBlock(block)
	if state != nil && state.SI != nil {
		for _, v := range state.SI {
			if v.State == ssComm.Deleted {
				lh.stateHandler.DeleteState(v.ChainID, v.Key)
			} else {
				lh.stateHandler.SetState(v.ChainID, v.Key, v.Value)
			}
		}
	}
	SetBlockHeight(block.BlockNum + 1)
	//把原始记录与新纪录保存起来
	SetState([]byte("test"), state)
	lh.dbHandler.BatchCommit()
	return nil
}

func GetContractState(chainID string, key []byte) ([]byte, error) {
	return lh.stateHandler.GetState(chainID, key)
}

func RollBackLedger(blockNum uint64) error {
	GetStateSet([]byte("test"))
	return nil
}

func GetBlock(blkNum uint64) (*bsComm.Block, error) {
	return lh.blockHandler.GetBlock(blkNum)
}

func GetBlockHash(blkNum uint64) ([]byte, error) {
	//todo
	//return msp.Hash256([]byte("todo"))
	return lh.blockHandler.GetBlockHash(blkNum)
	//return lh.blockHandler.GetBlockHash(blkNum)
}

//todo
func CalcBlockHash(block *bsComm.Block) ([]byte, error) {
	//return msp.Hash256([]byte("todo"))
	//return nil, nil
	return lh.blockHandler.CalcBlockHash(block)
}

func CalcHashRoot(hasher [][]byte) []byte {
	return merkle.SimpleHashFromByteslices(hasher)
}
