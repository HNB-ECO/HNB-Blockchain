package db

import(
	dbComm "HNB/db/common"
	lImpl "HNB/db/leveldbImpl"
	"HNB/config"
	"HNB/logging"
)


var DBLog logging.LogModule

const(
	LOGTABLE_DB string = "db"
)

func InitDB(dbType string) (dbComm.KVStore, error){
    DBLog = logging.GetLogIns()
	switch dbType {
	case "leveldb":
		var file string
		if config.Config.DBPath != ""{
			file = config.Config.DBPath
		}else{
			file = "./testDB"
		}
		DBLog.Info(LOGTABLE_DB, "leveldb file: " + file)
		ins, err := lImpl.NewLevelDBStore(file)
		if err != nil{
			return nil, err
		}
		return ins, nil
	}
	return nil, nil
}

