package common

import (
	"fmt"
	"time"
)

/**
 * @MethodName:
 * @Description: Tendermint 共识服务的配置，超时配置，空块设置等
 * @param
 * @author lenovo
 * @date 2018/4/26 14:48
 */

type ConsensusConfig struct {

	// 所有超时都是毫秒为单位
	// Delta 超时增量
	TimeoutNewRound      int `json:"timeoutNewRound,omitempty"`
	TimeoutPropose       int `json:"timeoutPropose,omitempty"`
	TimeoutProposeWait   int `json:"timeoutProposeWait,omitempty"`
	TimeoutPrevote       int `json:"timeoutPrevote,omitempty"`
	TimeoutPrevoteWait   int `json:"timeoutPrevoteWait,omitempty"`
	TimeoutPrecommit     int `json:"timeoutPrecommit,omitempty"`
	TimeoutPrecommitWait int `json:"timeoutPrecommitWait,omitempty"`
	TimeoutCommit        int `json:"timeoutCommit,omitempty"`

	// 是否跳过TimeoutCommit
	SkipTimeoutCommit         bool `json:"skipTimeoutCommit,omitempty"`
	CreateEmptyBlocks         bool `json:"createEmptyBlocks,omitempty"`
	CreateEmptyBlocksInterval int  `json:"createEmptyBlocksInterval,omitempty"`

	//Reactor sleep duration parameters are in milliseconds
	TimeoutWaitFortx int      `json:"peerGossipSleepDuration,omitempty"`
	BlkTimeout       int      `json:"blkTimeout,omitempty"`
	BgDemandTimeout  int      `json:"bgDemandTimeout,omitempty"`
	BftNum           int      `json:"bftNum,omitempty"`
	GeneBftName      []string `json:"geneBftName,omitempty"`
}

// WaitForTxs returns true if the consensus should wait for transactions before entering the propose step
func (cfg *ConsensusConfig) WaitForTxs() bool { // 已失效
	return !cfg.CreateEmptyBlocks || cfg.CreateEmptyBlocksInterval > 0
}

// EmptyBlocks returns the amount of time to wait before proposing an empty block or starting the propose timer if there are no txs available
func (cfg *ConsensusConfig) EmptyBlocksInterval() time.Duration {
	return time.Duration(cfg.CreateEmptyBlocksInterval) * time.Millisecond
}

// Propose returns the amount of time to wait for a proposal
func (cfg *ConsensusConfig) Propose(round int) time.Duration {
	return time.Duration(cfg.TimeoutPropose+cfg.TimeoutPropose*round) * time.Millisecond
}

// PeerGossipSleep returns the amount of time to sleep if there is nothing to send from the ConsensusReactor
func (cfg *ConsensusConfig) PeerGossipSleep() time.Duration { //失效
	/*return time.Duration(cfg.PeerGossipSleepDuration) * time.Millisecond*/
	return 5 * time.Millisecond
}

// Prevote returns the amount of time to wait for straggler votes after receiving any +2/3 prevotes
func (cfg *ConsensusConfig) Prevote(round int32) time.Duration { //TODO
	//return time.Duration(cfg.TimeoutPrevote+cfg.TimeoutPrevoteDelta*round) * time.Millisecond
	return 0
}

// Precommit returns the amount of time to wait for straggler votes after receiving any +2/3 precommits
func (cfg *ConsensusConfig) Precommit(round int) time.Duration {
	return time.Duration(cfg.TimeoutPrecommit+cfg.TimeoutPrecommit*round) * time.Millisecond
}

// Commit returns the amount of time to wait for straggler votes after receiving +2/3 precommits for a single block (ie. a commit).
func (cfg *ConsensusConfig) Commit(t time.Time) time.Time { //失效
	return t.Add(time.Duration(cfg.TimeoutCommit) * time.Millisecond)
}

func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		TimeoutPropose:            3000,
		TimeoutProposeWait:        500,
		TimeoutPrevote:            1000,
		TimeoutPrevoteWait:        500,
		TimeoutPrecommit:          1000,
		TimeoutPrecommitWait:      500,
		TimeoutCommit:             1000,
		SkipTimeoutCommit:         false,
		CreateEmptyBlocks:         true,
		CreateEmptyBlocksInterval: 0,
	}
}

func (cfg *ConsensusConfig) String() string {
	return fmt.Sprintf(`tdmConsConfig:{
	timeoutNewRound: %v
	timeoutPropose: %v
	timeoutProposeWait %v
	timeoutPrevote %v
	timeoutPrevoteWait %v
	timeoutPrecommit %v
	timeoutPrecommitWait %v
	timeoutCommit %v
	skipTimeoutCommit %v
	createEmptyBlocks %v
	createEmptyBlocksInterval %v
	timeoutWaitFortx %v
	blkTimeout %v
	bgDemandTimeout %v
	bftNum %v
	geneBftName %v
	}`,
		cfg.TimeoutNewRound,
		cfg.TimeoutPropose,
		cfg.TimeoutProposeWait,
		cfg.TimeoutPrevote,
		cfg.TimeoutPrevoteWait,
		cfg.TimeoutPrecommit,
		cfg.TimeoutPrecommitWait,
		cfg.TimeoutCommit,
		cfg.SkipTimeoutCommit,
		cfg.CreateEmptyBlocks,
		cfg.CreateEmptyBlocksInterval,
		cfg.TimeoutWaitFortx,
		cfg.BlkTimeout,
		cfg.BgDemandTimeout,
		cfg.BftNum,
		cfg.GeneBftName,
	)
}

func TestConsensusConfig() *ConsensusConfig {
	cfg := DefaultConsensusConfig()
	cfg.TimeoutPropose = 1000
	cfg.TimeoutProposeWait = 100
	cfg.TimeoutPrevote = 1000
	cfg.TimeoutPrevoteWait = 100
	cfg.TimeoutPrecommit = 1000
	cfg.TimeoutPrecommitWait = 100
	cfg.TimeoutCommit = 1000
	cfg.SkipTimeoutCommit = true

	return cfg
}
