package dbft

import (
	"bytes"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
)

type EpochQueryInfo struct {
	EpochNo       uint64 `json:"epoch_no"`
	DependEpochNo uint64 `json:"depend_epoch_no"`
	BeginNum      uint64 `json:"begin_num"`
	EndNum        uint64 `json:"end_num"`
	TargetBlkNum  int    `json:"target_blk_num"`
	WitnessList   string `json:"witness_list"`
}

type Epoch struct {
	EpochNo       uint64             `json:"epoch_no"`
	DependEpochNo uint64             `json:"depend_epoch_no"`
	BeginNum      uint64             `json:"begin_num"`
	EndNum        uint64             `json:"end_num"`
	TargetBlkNum  int                `json:"target_blk_num"`
	WitnessList   []*types.Validator `json:"witness_list"`
	IsToTarget    bool               `json:"is_to_target"`
}

func (eh *Epoch) IsExist(digestAddr util.HexBytes) bool {
	if len(digestAddr) == 0 {
		return false
	}

	for _, validator := range eh.WitnessList {
		if bytes.Equal(validator.Address, digestAddr) {
			return true
		}
	}
	return false
}

type EpochBlks struct {
	begin uint64
	end   uint64
}

func NewEpochBlks() *EpochBlks {
	return &EpochBlks{}
}

func (eb *Epoch) ReSetBeginBlk(height uint64) {
	eb.BeginNum = height
	eb.EndNum = height
}

func (eb *Epoch) SetBeginBlk(height uint64) {
	eb.BeginNum = height
}
func (eb *Epoch) SetEndBlk(height uint64) {
	eb.EndNum = height
}

func (eb *Epoch) GetBeginBlk() uint64 {
	return eb.BeginNum
}
func (eb *Epoch) GetEndBlk() uint64 {
	return eb.EndNum
}

func (eb *Epoch) IsBlksToTarget(targetCount int) bool {
	if eb.BeginNum > eb.EndNum {
		return false
	}
	if eb.EndNum-eb.BeginNum >= uint64(targetCount) {
		return true
	}
	return false
}
