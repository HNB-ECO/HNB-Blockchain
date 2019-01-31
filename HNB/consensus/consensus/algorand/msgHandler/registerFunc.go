package msgHandler

import (
	"HNB/consensus/algorand/types"
)

func (h *TDMMsgHandler) RegisterIsProposerFunc(isProposerFunc func(tdmMsgHander *TDMMsgHandler, height uint64, round int32) (bool, *types.Validator)) {
	h.isProposerFunc = isProposerFunc
}

func (h *TDMMsgHandler) RegisterGeneProposalBlkFunc(geneProposalBlkFunc func(tdmMsgHander *TDMMsgHandler) (block *types.Block, blockParts *types.PartSet)) {
	h.geneProposalBlkFunc = geneProposalBlkFunc
}

func (h *TDMMsgHandler) RegisterConsSucceedFunc(busiName string, consSucceedFunc func(tdmMsgHander *TDMMsgHandler, block *types.Block) error) {
	h.consSucceedFuncChain[busiName] = consSucceedFunc
}

func (h *TDMMsgHandler) RegisterValidateProposalBlkFunc(validateProposalBlkFunc func(tdmMsgHander *TDMMsgHandler, block *types.Block) bool) {
	h.validateProposalBlkFunc = validateProposalBlkFunc
}

func (h *TDMMsgHandler) RegisterStatusUpdatedFunc(busiName string, statusUpdatedFunc func(tdmMsgHander *TDMMsgHandler) error) {
	h.statusUpdatedFuncChain[busiName] = statusUpdatedFunc
}
