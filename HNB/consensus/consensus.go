package consensus

import (
	"HNB/consensus/solo"
	"HNB/consensus/algorand"
)

const (
	SOLO = "solo"
	ALGORAND = "algorand"
)

type ConsensusServer interface {
	Start()
	Stop()
}

func NewConsensusServer(consensusType string, chainId string)  ConsensusServer {
	if consensusType == "" {
		consensusType = SOLO
	}
	var consensus ConsensusServer
	switch consensusType {
	case SOLO:
		consensus = solo.NewSoloServer(chainId)
	case ALGORAND:
		consensus = algorand.NewAlgorandServer()
	}
	return consensus
}
