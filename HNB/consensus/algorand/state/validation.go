package state

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
)

func ValidateBlock(s State, b *types.Block) error {

	if err := b.ValidateBasic(); err != nil {
		return err
	}

	if b.BlockNum != s.LastBlockNum+1 {
		return fmt.Errorf("Wrong Block.Header.BlockNum. Expected %v, got %v", s.LastBlockNum+1, b.BlockNum)
	}

	if !b.LastBlockID.Equals(s.LastBlockID) {
		return fmt.Errorf("Wrong Block.Header.LastBlockID.  Expected %v, got %v", s.LastBlockID, b.LastBlockID)
	}
	newTxs := int64(len(b.Data.Txs))

	if b.TotalTxs != s.LastBlockTotalTx+newTxs {
		return fmt.Errorf("Wrong Block.Header.TotalTxs. Expected %v, got %v", s.LastBlockTotalTx+newTxs, b.TotalTxs)
	}

	if !bytes.Equal(b.ConsensusHash, s.ConsensusParams.Hash()) {
		return fmt.Errorf("Wrong Block.Header.ConsensusHash.  Expected %X, got %v", s.ConsensusParams.Hash(), b.ConsensusHash)
	}

	if !bytes.Equal(b.ValidatorsHash, s.Validators.Hash()) { //TODO ValidatorsHash
		return fmt.Errorf("tendermint: Wrong Block.Header.ValidatorsHash.  Expected %X, got %v "+
			"s.validators %v b.validators %v b.valLastHeightChanged %v", s.Validators.Hash(), b.ValidatorsHash, s.Validators, b.Validators, b.LastHeightChanged)
	}

	if b.BlockNum == 1 {
		if len(b.LastCommit.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no LastCommit precommits")
		}
	} else {
		if len(b.LastCommit.Precommits) != s.LastValidators.Size() {
			return fmt.Errorf("Invalid block commit size. Expected %v, got %v",
				s.LastValidators.Size(), len(b.LastCommit.Precommits))
		}
		err := s.LastValidators.VerifyCommitVoteSign(b.BlockNum-1, b.LastCommit)
		if err != nil {
			fmt.Println("validate vote signature err ", err)
			//return err
		}
	}
	return nil
}
