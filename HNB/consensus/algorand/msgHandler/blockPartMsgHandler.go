package msgHandler

import (
	"encoding/json"
	"fmt"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/merkle"
)

func (h *TDMMsgHandler) HandleBlockPartMsg(tdmMsg *cmn.TDMMessage) error {
	blkPartMsg := &cmn.BlockPartMessage{}
	err := json.Unmarshal(tdmMsg.Payload, blkPartMsg)
	if err != nil {
		return err
	}
	blkPart := h.buildTypeBlkPartFromNetMsg(blkPartMsg)

	_, err = h.addProposalBlockPart(blkPartMsg.Height, blkPart, true)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "HandleBlockPartMsg err %v", err)
		return nil
	}

	return nil
}

func (h *TDMMsgHandler) buildTypeBlkPartFromNetMsg(blkPartMsg *cmn.BlockPartMessage) *types.Part {
	return &types.Part{
		Proof: merkle.SimpleProof{Aunts: blkPartMsg.Part.Proof},
		Index: int(blkPartMsg.Part.Index),
		Bytes: blkPartMsg.Part.Bytes,
	}
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit, once we have the full block.
func (h *TDMMsgHandler) addProposalBlockPart(height uint64, part *types.Part, verify bool) (added bool, err error) {
	// Blocks might be reused, so round mismatch is OK
	if h.Height != height {
		return false, nil
	}

	// We're not expecting a block part.
	if h.ProposalBlockParts == nil {
		return false, nil // TODO: bad peer? Return error?
	}

	added, err = h.ProposalBlockParts.AddPart(part, verify)
	if err != nil {
		return added, err
	}
	if added && h.ProposalBlockParts.IsComplete() {
		// Added and completed!
		proposalBlk := &types.Block{}
		_, err = types.Codec.UnmarshalBinaryReader(h.ProposalBlockParts.GetReader(), proposalBlk, int64(100*1024*1024)) //最大的大小是100M   超过100M无法切割分批
		if err != nil {
			return true, err
		}
		// 校验VRFValue和VRFProof
		//proposer := proposalBlk.Proposer
		//VRFValue := proposalBlk.BlkVRFValue
		//VRFProof := proposalBlk.BlkVRFProof
		//
		//VRFBlkData := &vrf.VRFBlkData{
		//	PrevVrf:  h.LastCommitState.PrevVRFValue,
		//	BlockNum: proposalBlk.BlockNum,
		//}
		//
		//_, val := h.Validators.GetByAddress(proposer.Address)
		//pk := msp.StringToBccspKey(val.PubKeyStr)
		//if err != nil {
		//	return true, err
		//}
		if !h.validateProposalBlkFunc(h, proposalBlk) {
			return true, fmt.Errorf("proposal validate fail")
		}

		h.ProposalBlock = proposalBlk
		ConsLog.Debugf(LOGTABLE_CONS, "(blockPart 2 proposal block) ", h.ProposalBlock)

		if len(h.ProposalBlock.LastCommit.Precommits) > 0 {
			ConsLog.Debugf(LOGTABLE_CONS, "precommit vote time %v", h.ProposalBlock.LastCommit) //中间断掉节点的情况会报空指针 所以取消打印时间
		}

		ConsLog.Infof(LOGTABLE_CONS, "Received complete proposal block height:%v hash:%v lastCommit_hash:%v",
			h.ProposalBlock.BlockNum, h.ProposalBlock.Hash(), h.ProposalBlock.LastCommit.Hash())

		if h.Step == types.RoundStepPropose {
			h.enterPrevote(height, h.Round)
		} else if h.Step == types.RoundStepCommit {
			h.tryFinalizeCommit(height)
		}

		return true, nil
	}

	return added, nil
}
