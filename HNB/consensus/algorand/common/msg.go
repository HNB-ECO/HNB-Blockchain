package common

type PeerMessage struct {
	Msg         []byte
	Sender      uint64
	IsBroadCast bool
	Flag        int // 0 prepose 1 provote 2 precommit 3 blk
}

type TDMType int32

const (
	TDMType_MsgProposal          TDMType = 0
	TDMType_MsgVote              TDMType = 1
	TDMType_MsgBlockPart         TDMType = 2
	TDMType_MsgProposalHeartBeat TDMType = 3
	TDMType_MsgNewRoundStep      TDMType = 4
	TDMType_MsgCommitStep        TDMType = 5
	TDMType_MsgProposalPOL       TDMType = 6
	TDMType_MsgHasVote           TDMType = 7
	TDMType_MsgVoteSetMaj23      TDMType = 8
	TDMType_MsgVoteSetBits       TDMType = 9
	TDMType_MsgHeightReq         TDMType = 10
	TDMType_MsgHeihtResp         TDMType = 11
	TDMType_MsgBgAdvice          TDMType = 12
	TDMType_MsgBgDemand          TDMType = 13
)

type TDMMessage struct {
	Type    TDMType
	Payload []byte
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
	Height        uint64    `protobuf:"varint,1,opt,name=Height" json:"Height,omitempty"`
	Round         int32     `protobuf:"varint,2,opt,name=Round" json:"Round,omitempty"`
	VoteType      VoteType  `protobuf:"varint,3,opt,name=VoteType,enum=tdmRPC.VoteType" json:"VoteType,omitempty"`
	Index         int32     `protobuf:"varint,4,opt,name=Index" json:"Index,omitempty"`
	VotesBitArray *BitArray `protobuf:"bytes,5,opt,name=VotesBitArray" json:"VotesBitArray,omitempty"`
}