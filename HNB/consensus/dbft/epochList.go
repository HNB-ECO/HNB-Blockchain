package dbft

import (
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"sync"
)

type EpochListCache struct {
	sync.RWMutex
	list    map[uint64]*Epoch
	chainid string

	Logger logging.LogModule
}

func NewEpochList(chainid string) *EpochListCache {
	return &EpochListCache{
		list:    make(map[uint64]*Epoch, 0),
		chainid: chainid,
		Logger:  logging.GetLogIns(),
	}
}

//todo 需要读取数据库信息
func (elc *EpochListCache) isExist(epochNo uint64) bool {
	elc.RLock()
	defer elc.RUnlock()
	_, ok := elc.list[epochNo]
	if !ok {
		epochInfo, err := LoadEpochInfo(elc.chainid, epochNo)
		if err != nil {
			elc.Logger.Errorf(LOGTABLE_DBFT, "LoadEpochInfo err %s", err.Error())
			return false
		}
		if epochInfo == nil {
			return false
		}
	}
	return ok
}

func (elc *EpochListCache) GetEpoch(epochNo uint64) *Epoch {
	elc.Lock()
	defer elc.Unlock()
	epoch := elc.list[epochNo]
	if epoch == nil {
		epochInfo, err := LoadEpochInfo(elc.chainid, epochNo)
		if err != nil {
			elc.Logger.Errorf(LOGTABLE_DBFT, "LoadEpochInfo err %s", err.Error())
			return nil
		}
		if epochInfo == nil {
			return nil
		}
		epoch = epochInfo
		elc.list[epochNo] = epoch
		elc.Logger.Infof(LOGTABLE_DBFT, "LoadEpochInfo store epoch %d", epochInfo.EpochNo)
	}
	elc.Logger.Infof(LOGTABLE_DBFT, "EpochListCache %v", epoch)
	return epoch
}

func (elc *EpochListCache) SetEpoch(epoch *Epoch) error {
	if epoch == nil {
		return fmt.Errorf("epoch info is nil")
	}
	elc.Lock()
	elc.list[epoch.EpochNo] = epoch
	elc.Logger.Infof(LOGTABLE_DBFT, "store epoch %d", epoch.EpochNo)
	elc.Unlock()
	return nil
}

func (elc *EpochListCache) DelEpoch(epochNo uint64) error {
	elc.Lock()
	delete(elc.list, epochNo)
	elc.Unlock()
	return nil
}

func (elc *EpochListCache) CleanEpoch(epochNo uint64) error {
	var targetEpochNo uint64
	if epochNo < 50 {
		return nil
	}
	targetEpochNo = epochNo - 50
	elc.Logger.Infof(LOGTABLE_DBFT, "CleanEpoch less than %d", targetEpochNo)

	elc.Lock()
	for _, epoch := range elc.list {
		if epoch == nil {
			return fmt.Errorf("epoch is nil")
		}
		if epoch.EpochNo < targetEpochNo {
			elc.Logger.Infof(LOGTABLE_DBFT, "CleanEpoch epoch %d", epoch.EpochNo)
			delete(elc.list, epoch.EpochNo)
		}
	}
	elc.Unlock()
	return nil
}
