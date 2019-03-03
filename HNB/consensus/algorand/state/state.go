package state

import (
	"bytes"
	"time"

	"encoding/json"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
)

// database keys
var (
	stateKey = []byte("stateKey")
)

type State struct {

	// LastBlockNum=0 at genesis (ie. block(H=0) does not exist)
	LastBlockNum     uint64
	LastBlockTotalTx int64
	LastBlockID      types.BlockID
	LastBlockTime    time.Time

	// LastValidators is used to validate block.LastCommit.
	// Validators are persisted to the database separately every time they change,
	// so we can query for historical validator sets.
	// Note that if s.LastBlockNum causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockNum + 1
	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged uint64

	// Consensus parameters used for validating blocks.
	// Changes returned by EndBlock and updated after Commit.
	ConsensusParams                  types.ConsensusParams
	LastHeightConsensusParamsChanged int64

	// Merkle root of the results from executing prev block
	LastResultsHash []byte

	// 上个块的hash值
	PreviousHash []byte
	// 上个块的VRFValue
	PrevVRFValue []byte

	// 上个块的VRFProof
	PrevVRFProof []byte
	// The latest AppHash we've received from calling abci.Commit()
	//AppHash []byte
}

// Copy makes a copy of the State for mutating.
func (s State) Copy() State {
	return State{
		LastBlockNum:     s.LastBlockNum,
		LastBlockTotalTx: s.LastBlockTotalTx,
		LastBlockID:      s.LastBlockID,
		LastBlockTime:    s.LastBlockTime,

		Validators:                  s.Validators.Copy(),
		LastValidators:              s.LastValidators.Copy(),
		LastHeightValidatorsChanged: s.LastHeightValidatorsChanged,

		//ConsensusParams:                  s.ConsensusParams,
		LastHeightConsensusParamsChanged: s.LastHeightConsensusParamsChanged,

		//AppHash: s.AppHash,

		LastResultsHash: s.LastResultsHash,
	}
}

// Equals returns true if the States are identical.
func (s State) Equals(s2 State) bool {
	sbz, s2bz := s.Bytes(), s2.Bytes()
	return bytes.Equal(sbz, s2bz)
}

// Bytes serializes the State using go-amino.
func (s State) Bytes() []byte {
	bytes, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	return bytes
}

// IsEmpty returns true if the State is equal to the empty State.
func (s State) IsEmpty() bool {
	return s.Validators == nil // XXX can't compare to Empty
}

// GetValidators returns the last and current validator sets.
func (s State) GetValidators() (last *types.ValidatorSet, current *types.ValidatorSet) {
	return s.LastValidators, s.Validators
}

// Create a block from the latest state
// MakeBlock builds a block with the given txs and commit from the current state.
func (s State) MakeBlock(blkNum uint64, txs []types.Tx, commit *types.Commit) (*types.Block, *types.PartSet) {
	// build base block
	block := types.MakeBlock(blkNum, txs, commit)
	//收集交易总数
	block.TotalTxs = s.LastBlockTotalTx + block.NumTxs
	//前块信息
	block.LastBlockID = s.LastBlockID
	block.ValidatorsHash = s.Validators.Hash()
	block.ConsensusHash = s.ConsensusParams.Hash()

	var changeHeight = s.LastHeightValidatorsChanged
	valInfo := &types.ValidatorInfo{
		LastHeightChanged: s.LastHeightValidatorsChanged,
	}

	if changeHeight == blkNum {
		valInfo.Validators = s.Validators.Copy()
	}

	block.ValidatorInfo = valInfo
	return block, block.MakePartSet(1024 * 1024)
}

func (s State) MakeBlockVRF(material *types.BlkMaterial) (*types.Block, *types.PartSet) {
	// build base block
	block := types.MakeBlock(material.Height, material.Txs, material.Commit)

	// fill header with state data
	//block.ChainID = s.ChainID
	//block.NumTxs = int64(material.NumTxs)
	block.TotalTxs = s.LastBlockTotalTx + block.NumTxs
	block.LastBlockID = s.LastBlockID
	block.ValidatorsHash = s.Validators.Hash()
	//block.AppHash = s.AppHash
	block.ConsensusHash = s.ConsensusParams.Hash()
	//block.LastResultsHash = s.LastResultsHash
	block.BlkVRFValue = material.BlkVRFValue
	block.BlkVRFProof = material.BlkVRFProof

	var changeHeight = s.LastHeightValidatorsChanged
	valInfo := &types.ValidatorInfo{
		LastHeightChanged: s.LastHeightValidatorsChanged,
	}
	if changeHeight == material.Height {
		valInfo.Validators = s.Validators.Copy()
	}

	valInfo.Validators.Proposer = material.Proposer
	block.ValidatorInfo = valInfo

	return block, block.MakePartSet(1024 * 1024)
}

func (s State) MakeBlockDPoS(material *types.BlkMaterial) (*types.Block, *types.PartSet) {
	// build base block
	block := types.MakeBlock(material.Height, material.Txs, material.Commit)

	// fill header with state data
	block.NumTxs = int64(material.NumTxs)
	block.TotalTxs = s.LastBlockTotalTx + block.NumTxs
	block.LastBlockID = s.LastBlockID
	block.ValidatorsHash = s.Validators.Hash()
	block.ConsensusHash = s.ConsensusParams.Hash()

	valInfo := &types.ValidatorInfo{
		LastHeightChanged: s.LastHeightValidatorsChanged,
	}

	valInfo.Validators = s.Validators.Copy()
	valInfo.Validators.Proposer = material.Proposer
	block.ValidatorInfo = valInfo

	return block, block.MakePartSet(1024 * 1024)
}
