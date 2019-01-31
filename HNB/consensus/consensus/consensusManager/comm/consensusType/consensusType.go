package consensusType

//共识类型
type ConsensusType uint8

const (
	PoW        = ConsensusType(0x01)
	Tendermint = ConsensusType(0x02)
	PoS        = ConsensusType(0x03)
	DBFT       = ConsensusType(0x04)
)

// String returns a string
func (cons ConsensusType) String() string {
	switch cons {
	case PoW:
		return "PoW"
	case Tendermint:
		return "Tendermint"
	case PoS:
		return "PoS"
	case DBFT:
		return "DBFT"
	default:
		return "UnknownCons"
	}
}

type ConsensusMsg struct {
	Payload []byte `json:"payload"`
	Type    int    `json:"type"`
}
