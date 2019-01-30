package ledger

import (
	"errors"
	txComm "github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/blockStore"
	bsComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/blockStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/merkle"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/nonceStore"
	nnComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/nonceStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/stateStore"
	ssComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/stateStore/common"
	wgStore "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/wrongStore"
	wgComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/wrongStore/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
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
	nonceHandler nnComm.NonceStore
	wrongHandler wgComm.WrongIndexStore
}

var lh *ledgerHandler

func InitLedger(db dbComm.KVStore, blockDB dbComm.KVStore) error {
	LedgerLog = logging.GetLogIns()
	LedgerLog.Info(LOGTABLE_LEDGER, "Init ledger")
	lh = &ledgerHandler{}
	lh.dbHandler = db
	lh.blockHandler = blockStore.NewBlockStore(blockDB)
	lh.stateHandler = stateStore.NewStateStore(db)
	lh.nonceHandler = nonceStore.NewNonceStore(db)
	lh.wrongHandler = wgStore.NewWrongStore(db)

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

func WriteLedger(block *bsComm.Block, state *ssComm.StateSet, nonce *nnComm.NonceSet) error {
	h, err := GetBlockHeight()
	if err != nil {
		return nil
	}

	if block.Header.BlockNum != h {
		return errors.New("height invalid")
	}

	lh.dbHandler.NewBatch()
	defer func() {
		if err != nil {
			lh.dbHandler.Close()
		}
	}()

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
	err = SetBlockHeight(block.Header.BlockNum + 1)
	if err != nil {
		return err
	}

	err = SetHashIndex(block)
	if err != nil {
		return err
	}

	if nonce != nil && nonce.NI != nil {
		for _, v := range nonce.NI {
			LedgerLog.Debugf(LOGTABLE_LEDGER, "ledger set nonce addr:%v, nonce:%v", v.Key, v.Nonce)
			lh.nonceHandler.SetNonce(v.Key, v.Nonce)
		}
	}

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
	return lh.blockHandler.GetBlockHash(blkNum)
}

func CalcBlockHash(block *bsComm.Block) ([]byte, error) {
	return lh.blockHandler.CalcBlockHash(block)
}

func CalcHashRoot(hasher [][]byte) []byte {
	return merkle.SimpleHashFromByteslices(hasher)
}

func SetNonce(address txComm.Address, nonce uint64) error {
	return lh.nonceHandler.SetNonce(address, nonce)
}
func GetNonce(address txComm.Address) (uint64, error) {
	return lh.nonceHandler.GetNonce(address)
}

func GetTransaction(blkNum uint64, offset uint32) (*txComm.Transaction, error) {
	blk, err := GetBlock(blkNum)
	if err != nil {
		return nil, err
	}
	if blk == nil {
		return nil, errors.New("get transaction blk invalid")
	}

	txLen := len(blk.Txs)

	if uint32(txLen) < offset+1 {
		return nil, errors.New("txLen or offset invalid")
	}

	tx := blk.Txs[offset]

	return tx, nil
}

func GetWrongIndex(txid []byte) (string, error) {
	return lh.wrongHandler.GetWrongIndex(txid)
}

func SetWrongIndex(txid []byte, reason string) error {
	return lh.wrongHandler.SetWrongIndex(txid, reason)
}
