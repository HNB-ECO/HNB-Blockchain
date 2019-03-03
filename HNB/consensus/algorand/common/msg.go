package common

import (
	lgCom "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/blockStore/common"
)

type PeerMessage struct {
	Msg    []byte
	PeerID uint64
	//公钥信息
	Sender      []byte
	IsBroadCast bool
	Flag        int // 0 prepose 1 provote 2 precommit 3 blk
}

type TDMType int32

const (
	TDMType_MsgProposal            TDMType = 0
	TDMType_MsgVote                TDMType = 1
	TDMType_MsgBlockPart           TDMType = 2
	TDMType_MsgProposalHeartBeat   TDMType = 3
	TDMType_MsgNewRoundStep        TDMType = 4
	TDMType_MsgCommitStep          TDMType = 5
	TDMType_MsgProposalPOL         TDMType = 6
	TDMType_MsgHasVote             TDMType = 7
	TDMType_MsgVoteSetMaj23        TDMType = 8
	TDMType_MsgVoteSetBits         TDMType = 9
	TDMType_MsgHeightReq           TDMType = 10
	TDMType_MsgHeihtResp           TDMType = 11
	TDMType_MsgBgAdvice            TDMType = 12
	TDMType_MsgBgDemand            TDMType = 13
	TDMType_MsgBinaryBlockHashReq  TDMType = 14
	TDMType_MsgBinaryBlockHashResp TDMType = 15
	TDMType_MsgCheckFork           TDMType = 16
)

type TDMMessage struct {
	Type      TDMType
	Payload   []byte
	Signature []byte
}

type BlockPartMessage struct {
	Height uint64
	Round  int32
	Part   *Part
}

type Part struct {
	Index uint32
	Bytes []byte
	Proof [][]byte
	Hash  []byte
}

type VoteType int32

const (
	VoteType_Prevote   VoteType = 0
	VoteType_Precommit VoteType = 1
)

type HasVoteMessage struct {
	Height        uint64
	Round         int32
	VoteType      VoteType
	Index         int32
	VotesBitArray *BitArray
}

type PartSetHeader struct {
	Total uint32
	Hash  []byte
}

type BlockID struct {
	Hash        []byte
	PartsHeader *PartSetHeader
}

type ProposalMessage struct {
	Height           uint64
	Round            int32
	Timestamp        uint64
	BlockPartsHeader *PartSetHeader
	PoLRound         int32
	PoLBlockID       *BlockID
	Signature        []byte
}

type ProposalHeartbeatMessage struct {
	ValidatorAddress []byte
	ValidatorIndex   uint32
	Height           uint64
	Round            int32
	Sequence         uint32
}

type NewRoundStepMessage struct {
	Height                uint64
	Round                 int32
	Step                  uint32
	SecondsSinceStartTime uint32
	LastCommitRound       int32
}

type ProposalPOLMessage struct {
	Height           uint64
	ProposalPOLRound int32
	ProposalPOL      *BitArray
}

type VoteMessage struct {
	ValidatorAddress []byte
	ValidatorIndex   int32
	Height           uint64
	Round            int32
	Timestamp        uint64
	VoteType         VoteType
	BlockID          *BlockID
	Signature        []byte
}

type BftGroupSwitchAdvice struct {
	DigestAddr  []byte //本节点的地址
	BftGroupNum uint64
	Height      uint64
	Hash        []byte
}

type BftGroupSwitchDemand struct {
	DigestAddr         []byte //本轮次提案人地址？
	BftGroupNum        uint64
	BftGroupCandidates []*BftGroupSwitchAdvice
	NewBftGroup        []*BftGroupSwitchAdvice
	VRFValue           []byte
	VRFProof           []byte
}

type HeightReq struct {
	IsFromBg bool
	Version  uint64
}

type HeightResp struct {
	BlockNum uint64
	Header   *lgCom.Header
	IsFromBg bool
	Version  uint64
}

type BinaryBlockHashReq struct {
	ReqId         string   `json:"reqId,omitempty"`
	ChainId       string   `json:"chainId,omitempty"`
	BlockNumSlice []uint64 `json:"blockNumSlice,omitempty"`
}

type BinaryBlockHashResp struct {
	ReqId          string   `json:"reqId,omitempty"`
	BlockHashSlice [][]byte `json:"blockHashSlice,omitempty"`
}

type CheckFork struct {
	BftGroupNum uint64 `json:"bftGroupNum,omitempty"`
	Height      uint64 `json:"height,omitempty"`
	Hash        []byte `json:"hash,omitempty"`
}

func (m *BinaryBlockHashResp) GetReqId() string {
	if m != nil {
		return m.ReqId
	}
	return ""
}

func (m *BinaryBlockHashResp) GetBlockHashSlice() [][]byte {
	if m != nil {
		return m.BlockHashSlice
	}
	return nil
}
func (m *BinaryBlockHashReq) GetReqId() string {
	if m != nil {
		return m.ReqId
	}
	return ""
}

func (m *BinaryBlockHashReq) GetBlockNumSlice() []uint64 {
	if m != nil {
		return m.BlockNumSlice
	}
	return nil
}
