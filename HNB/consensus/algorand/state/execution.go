package state

import  (

     "HNB/consensus/algorand/types"
)

type BlockExecutor struct {
	// save state, validators, consensus params, abci responses here
	//db dbm.DB

}

func (blockExec *BlockExecutor) ValidateBlock(s State, block *types.Block) error {
	return ValidateBlock(s, block)
}