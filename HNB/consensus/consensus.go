package consensus

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/dbft"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/solo"
)

const (
	SOLO     = "solo"
	ALGORAND = "algorand"
	DPoS     = "dpos"
)

type ConsensusServer interface {
	Start()
	Stop()
}

func NewConsensusServer(consensusType string, chainId string) ConsensusServer {
	if consensusType == "" {
		consensusType = SOLO
	}
	var consensus ConsensusServer
	switch consensusType {
	case SOLO:
		consensus = solo.NewSoloServer(chainId)
	case ALGORAND:
		consensus = algorand.NewAlgorandServer()
	case DPoS:
		consensus = dbft.NewDBFTServer()
	}
	return consensus
}
