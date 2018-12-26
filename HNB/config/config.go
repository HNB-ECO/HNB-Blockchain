package config

import (
	"HNB/util"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

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
	TimeoutWaitFortx int `json:"peerGossipSleepDuration,omitempty"`
	BlkTimeout       int `json:"blkTimeout,omitempty"`
	BgDemandTimeout  int `json:"bgDemandTimeout,omitempty"`
	BftNum           int `json:"bftNum,omitempty"`
	//GeneBftName      []string `json:"geneBftName,omitempty"`

	// 初始bft共识组 由peerID+"_"+orgID对应的节点组成
	GeneBftGroup []*GenesisValidator `json:"geneBftGroup,omitempty"`
	//初始化全部的验证者
	GeneValidators []*GenesisValidator `json:"geneValidators,omitempty"`
	EpochPeriod    int                 `json:"epochPeriod"`
}

type GenesisValidator struct {
	// 公钥字符串
	PubKeyStr string `json:"PubKeyStr,omitempty"`
	// 选取提案人proposer的权重
	Power int `json:"power,omitempty"`
	// 验证节点名称
	//Name string `json:"name,omitempty"`
	// 公钥算法类型
	//AlgType uint16 `json:"algType,omitempty"`
	//公钥信息
	//PeerID  string `json:"peerID,omitempty"`
}

type AllConfig struct {
	Log                       LogConfig `json:"logConfig"`
	SeedList                  []string  `json:"seedList"`
	MaxConnOutBound           uint      `json:"maxConnOutBound"`
	MaxConnInBound            uint      `json:"maxConnInBound"`
	EnableConsensus           bool      `json:"enableConsensus"`
	MaxConnInBoundForSingleIP uint      `json:"singleIP"`
	SyncPort                  uint16    `json:"syncPort"`
	ConsPort                  uint16    `json:"consPort"`
	RestPort                  uint16    `json:"restPort"`
	GPRCPort                  uint16    `json:"grpcPort"`
	IsPeersTLS                bool      `json:"isTLS"`
	IsServerTLS               bool      `json:"isVisitTLS"`
	TlsKeyPath                string    `json:"tlsKeyPath"`
	TlsCertPath               string    `json:"tlsCertPath"`
	IsBlkSQL                  bool      `json:"isBlkSQL"`
	SQLIP                     string    `json:"sqlIP"`
	UserName                  string    `json:"username"`
	Password                  string    `json:"passwd"`
	DBPath                    string    `json:"dbPath"`
	//algorand config
	KetPairPath     string `json:"ketPairPath"`
	ConsensusConfig `json:"consConfig"`
	// 创世块时间戳
	GenesisTime time.Time `json:"genesisTime"`
	// 接收共识消息管道长度
	RecvMsgChan int `json:"recvMsgChan,omitempty"`
}

type LogConfig struct {
	Path  string `json:"path"`
	Level string `json:"level"`
}

var Config = NewConfig()

func NewConfig() *AllConfig {
	c := &AllConfig{}
	return c
}

func LoadConfig(path string) {
	if path == "" {
		return
	}

	if util.PathExists(path) == false {
		fmt.Println("config path is missing")
		return
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic("config load fail: " + err.Error())
	}

	datajson := []byte(data)

	err = json.Unmarshal(datajson, Config)
	if err != nil {
		panic("config load fail: " + err.Error())
	}
}

