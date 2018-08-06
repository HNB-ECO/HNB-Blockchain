package config

import (
	"HNB/util"
	"fmt"
	"io/ioutil"
	"encoding/json"
)

type AllConfig struct{
	Log LogConfig `json:"logConfig"`
	SeedList      []string  `json:"seedList"`
	MaxConnOutBound uint `json:"maxConnOutBound"`
	MaxConnInBound uint  `json:"maxConnInBound"`
	EnableConsensus bool `json:"enableConsensus"`
	MaxConnInBoundForSingleIP uint `json:"singleIP"`
	SyncPort uint16 `json:"syncPort"`
	ConsPort uint16 `json:"consPort"`
	RestPort uint16 `json:"restPort"`
	IsTLS bool  `json:"isTLS"`
}

type LogConfig struct {
	Path string `json:"path"`
	Level string `json:"level"`
}


var Config  = NewConfig()

func NewConfig() *AllConfig{
	c := &AllConfig{}
	//c.Log.Path = "./"
	//c.Log.Level = "info"
	return c
}

func LoadConfig(path string){
	if path == ""{
		return
	}

	if util.PathExists(path) == false{
		fmt.Println("config path is missing")
		return
	}

	data, err := ioutil.ReadFile(path)
	if err != nil{
		panic("config load fail: " + err.Error())
	}

	datajson := []byte(data)

	err = json.Unmarshal(datajson, Config)
	if err != nil{
		panic("config load fail: " + err.Error())
	}
}

