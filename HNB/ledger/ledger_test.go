package ledger

import(
	"testing"
	"HNB/ledger/blockStore/common"
	ssComm "HNB/ledger/stateStore/common"
)

func Test1(t *testing.T){
	err := InitLedger()
	if err != nil{
		t.Error(err.Error())
	}

	blk := &common.Block{}
	blk.ChainID = "c"
	blk.BlockNum = 0

	states := &ssComm.StateSet{}
	states.SI = make([]*ssComm.StateItem, 2)

	state1 := &ssComm.StateItem{}
	state1.Key = []byte("first")
	state1.Value = []byte("first value")
	state1.State = ssComm.Changed

	state2 := &ssComm.StateItem{}
	state2.Key = []byte("second")
	state2.Value = []byte("second value")
	state2.State = ssComm.Changed

	states.SI[0] = state1
	states.SI[1] = state2

	err = WriteLedger(blk, states)
	if err != nil{
		t.Error(err.Error())
	}

	h, err := GetBlockHeight()
	if err != nil{
		t.Error(err.Error())
	}

	if h != 1 {
		t.Error("height invalid")
	}

	c, err := GetContractState("c", []byte("second"))
	if err != nil{
		t.Error(err.Error())
	}
	if string(c) != "second value"{
		t.Error("value invalid")
	}



}
