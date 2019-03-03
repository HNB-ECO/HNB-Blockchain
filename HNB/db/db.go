package db

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/config"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	lImpl "github.com/HNB-ECO/HNB-Blockchain/HNB/db/leveldbImpl"
	mImpl "github.com/HNB-ECO/HNB-Blockchain/HNB/db/mysqlImpl"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
)

var DBLog logging.LogModule

const (
	LOGTABLE_DB string = "db"
)

var KVDB dbComm.KVStore

func InitKVDB(dbType string) (dbComm.KVStore, error) {
	DBLog = logging.GetLogIns()
	switch dbType {
	case "leveldb":
		var file string
		if config.Config.DBPath != "" {
			file = config.Config.DBPath
		} else {
			file = "./testDB"
		}
		DBLog.Info(LOGTABLE_DB, "leveldb file: "+file)
		ins, err := lImpl.NewLevelDBStore(file)
		if err != nil {
			return nil, err
		}
		KVDB = ins
		return ins, nil
	}
	return nil, nil
}

func InitSQLDB() (dbComm.KVStore, error) {

	ins, err := mImpl.NewMySQLStore(config.Config.SQLIP,
		config.Config.UserName, config.Config.Password)

	if err != nil {
		return nil, err
	}
	//创建块表
	err = ins.NewBlockTable()
	if err != nil {
		return nil, err
	}
	return ins, nil
}
