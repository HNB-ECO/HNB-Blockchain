package common

type DPoSType int32

const (
	DPoSType_MsgEpochChange DPoSType = 0
	DPoSType_MsgNewEpoch    DPoSType = 1
)

type DPoSMessage struct {
	Type      DPoSType `json:"Type,omitempty"`
	ChainId   string   `json:"ChainId,omitempty"`
	Payload   []byte   `json:"Payload,omitempty"`
	Signature []byte   `json:"Signature,omitempty"`
}

type EpochChange struct {
	EpochNo       uint64 `json:"epochNo,omitempty"`
	DependEpochNo uint64 `json:"dependEpochNo,omitempty"`
	DigestAddr    []byte `json:"digestAddr,omitempty"`
	Height        uint64 `json:"height,omitempty"`
	Hash          []byte `json:"hash,omitempty"`
	IsTimeout     bool   `json:"isTimeout,omitempty"`
}

type NewEpoch struct {
	EpochNo       uint64 `json:"epochNo,omitempty"`
	DependEpochNo uint64 `json:"dependEpochNo,omitempty"`
	DigestAddr    []byte `json:"digestAddr,omitempty"`
	Witnesss      []byte `json:"witnesss,omitempty"`
	Begin         uint64 `json:"begin,omitempty"`
	End           uint64 `json:"end,omitempty"`
}
