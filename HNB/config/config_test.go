package config

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	ac := &AllConfig{}
	ac.Log.Level = "debug"
	ac.Log.Path = "\\"
	ac.SeedList = []string{"127.0.0.1:7710"}
	ac.MaxConnOutBound = 10
	ac.MaxConnInBound = 100
	ac.EnableConsensus = true
	ac.MaxConnInBoundForSingleIP = 100
	ac.SyncPort = 7720
	ac.ConsPort = 7710
	ac.RestPort = 7711
	ac.IsPeersTLS = false
	ac.DBPath = "\\"
	ac.GenesisTime = time.Now()
	ac.RecvMsgChan = 10
	ac.ConsensusConfig.TimeoutNewRound = 300
	ac.ConsensusConfig.TimeoutPropose = 3000
	ac.ConsensusConfig.TimeoutProposeWait = 500
	ac.ConsensusConfig.TimeoutPrevote = 3000
	ac.ConsensusConfig.TimeoutPrevoteWait = 500
	ac.ConsensusConfig.TimeoutPrecommit = 3000
	ac.ConsensusConfig.TimeoutPrecommitWait = 500
	ac.ConsensusConfig.TimeoutCommit = 1000

	ac.ConsensusConfig.SkipTimeoutCommit = false
	ac.ConsensusConfig.CreateEmptyBlocks = false
	ac.ConsensusConfig.CreateEmptyBlocksInterval = -1

	ac.ConsensusConfig.TimeoutWaitFortx = 500
	ac.ConsensusConfig.BlkTimeout = 5
	ac.ConsensusConfig.BgDemandTimeout = 1
	ac.ConsensusConfig.BftNum = 4
	gv := &GenesisValidator{}
	gv.PubKeyStr = "pubxxx"
	gv.Power = 1

	ac.ConsensusConfig.GeneValidators = []*GenesisValidator{gv, gv, gv, gv}
	ac.ConsensusConfig.GeneBftGroup = []*GenesisValidator{gv, gv, gv, gv}

	m, _ := json.Marshal(ac)
	fmt.Printf("%s\n", string(m))
}

