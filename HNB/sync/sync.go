package sync

import (
	"fmt"
	"sync"
	"time"

	appComm "github.com/HNB-ECO/HNB-Blockchain/HNB/appMgr/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	syncComm "github.com/HNB-ECO/HNB-Blockchain/HNB/sync/common"
)

var syncLogger logging.LogModule
var sh *SyncHandler

const (
	LOGTABLE_SYNC string = "sync"
)

type syncChainHandler struct {
	chainID string

	timeout int
	sync.RWMutex
	chainSyncVersion uint64
	syncBlockChannel chan *syncComm.ResponseBlocks
	syncInfo         *syncComm.SyncInfo
	sh               *SyncHandler
	notifyHandler    syncComm.NotifyFunc
	exitTask         chan struct{}
	interval         map[uint64]int

	valLock sync.RWMutex
}

type SyncHandler struct {
	timeout           int
	reqMaxBlocksCount uint
	syncHandler       map[string]*syncChainHandler
	syncReq           chan *blockSyncReq
}

func (sh *SyncHandler) GetSyncInfo(chainID string) (*syncComm.SyncInfo, error) {

	sch := sh.getSyncHandlerByChainID(chainID)

	if sch == nil {
		return nil, fmt.Errorf("chainID(%s = nil)", chainID)
	}

	return sch.syncInfo, nil
}

func NewSync() *SyncHandler {
	sh = &SyncHandler{}
	syncLogger = logging.GetLogIns()
	return sh
}

func (sh *SyncHandler) Start() {

	maxSyncBlocks := 5
	if maxSyncBlocks > 30 || maxSyncBlocks == 0 {
		panic(fmt.Sprintf("(sync blk).(%d > max(30))", maxSyncBlocks))
	}

	timeout := 30
	if timeout <= 0 || timeout >= 100 {
		sh.timeout = 30
	} else {
		sh.timeout = timeout
	}
	syncLogger.Infof(LOGTABLE_SYNC, "(sync).(timeout=%d)", sh.timeout)

	sh.reqMaxBlocksCount = uint(maxSyncBlocks)

	sh.syncHandler = make(map[string]*syncChainHandler)

	p2pNetwork.RegisterSyncNotify(sh.HandlerMessage)

	sh.syncReq = make(chan *blockSyncReq, 20)
	//
	//chains, err := lg.GetChainsInfo()
	//if err != nil {
	//	panic(err)
	//}
	//length := len(chains)
	//
	//for i := 0; i < length; i++ {
	//	sh.addSyncHandler(chains[i].ChainID)
	//}
	sh.addSyncHandler(appComm.HNB)

	//for j := 0; j < 3; j++ {
	go sh.blockSyncThread()
	//}

	time.Sleep(400 * time.Millisecond)
}

func (sh *SyncHandler) addSyncHandler(chainID string) *syncChainHandler {

	sh.syncHandler[chainID] = &syncChainHandler{
		chainID:          chainID,
		chainSyncVersion: 1,
		timeout:          sh.timeout,
		syncBlockChannel: make(chan *syncComm.ResponseBlocks, 50),
		syncInfo:         &syncComm.SyncInfo{chainID, 0, 0, 0, 0},
		sh:               sh,
		exitTask:         make(chan struct{}, 1),
		interval:         make(map[uint64]int),
	}

	return sh.syncHandler[chainID]
}

func (sh *SyncHandler) getMaxSyncBlockCount() uint {
	return sh.reqMaxBlocksCount
}

func (sh *SyncHandler) getSyncHandlerByChainID(chainID string) *syncChainHandler {

	sch, ok := sh.syncHandler[chainID]
	if ok {
		return sch
	}

	return nil
}
