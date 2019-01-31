package msgHandler

import (
	"HNB/appMgr"
	appComm "HNB/appMgr/common"
	"HNB/config"
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/state"
	"HNB/consensus/algorand/types"
	"HNB/ledger"
	bsComm "HNB/ledger/blockStore/common"
	"HNB/logging"
	"HNB/msp"
	"HNB/p2pNetwork"
	psync "HNB/sync/common"
	"HNB/txpool"
	"HNB/util"
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"
)

const CHAINID = "hgs"

var msgQueueSize = 1000
var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
	ErrInvalidProposalPOLRound  = errors.New("Error invalid proposal POL round")
	ErrAddingVote               = errors.New("Error adding vote")
	ErrVoteHeightMismatch       = errors.New("Error vote height mismatch")
)

type TDMMsgHandler struct {
	cmn.BaseService
	ID                 []byte
	digestAddr         util.HexBytes
	PeerMsgQueue       chan *cmn.PeerMessage
	InternalMsgQueue   chan *cmn.PeerMessage
	EventMsgQueue      chan *cmn.PeerMessage
	SyncMsgQueue       chan *cmn.PeerMessage
	ForkSearchMsgQueue chan *cmn.PeerMessage

	TxsAvailable  chan uint64
	timeoutTicker TimeoutTicker
	doPrevote     func(height uint64, round int32) uint64
	blockExec     *state.BlockExecutor

	mtx sync.Mutex
	types.RoundState
	LastCommitState  state.State //state for height-1
	timeState        *TimeStatistic
	done             chan struct{}
	allRoutineExitWg sync.WaitGroup
	writeBlock       func(blk *bsComm.Block) error
	syncHandler       *SyncHandler
	recvSyncChan      chan *psync.SyncNotify
	syncTimer         *time.Timer
	syncTimeout       time.Duration
	cacheSyncBlkCount map[string]*SyncBlkCount
	isSyncStatus      *cmn.MutexBool
	IsVoteBroadcast     bool
	heightReq           *HeightReq
	heightReqOutBg      *HeightReqOutBg
	checkFork           *CheckFork
	forkSearchWorker    *ForkSearchWorker
	IsContinueConsensus bool
	JustStarted         bool

	isProposerFunc          func(tdmMsgHander *TDMMsgHandler, height uint64, round int32) (bool, *types.Validator) //判断本轮是否为提案人
	geneProposalBlkFunc     func(tdmMsgHander *TDMMsgHandler) (block *types.Block, blockParts *types.PartSet)      //生成提案块方法
	validateProposalBlkFunc func(tdmMsgHander *TDMMsgHandler, block *types.Block) bool                             //验证块提案是否合法
	consSucceedFuncChain    map[string]func(tdmMsgHander *TDMMsgHandler, block *types.Block) error                 //共识成功后执行方法链，k-方法实现业务简称，v-方法实现
	statusUpdatedFuncChain  map[string]func(tdmMsgHander *TDMMsgHandler) error                                     //状态更新完之后执行方法链，k-方法实现业务简称，v-方法实现

}

const (
	proposalHeartbeatIntervalSeconds = 2
	CONSENSUS                        = "CONSENSUS"
)

var ConsLog logging.LogModule

const (
	LOGTABLE_CONS string = "consensus"
)

func NewTDMMsgHandler(lastCommitState state.State) (*TDMMsgHandler, error) {
	msgHandler := &TDMMsgHandler{
		timeoutTicker:      NewTimeoutTicker(),
		PeerMsgQueue:       make(chan *cmn.PeerMessage, msgQueueSize),
		InternalMsgQueue:   make(chan *cmn.PeerMessage, msgQueueSize),
		EventMsgQueue:      make(chan *cmn.PeerMessage, msgQueueSize),
		SyncMsgQueue:       make(chan *cmn.PeerMessage, msgQueueSize),
		ForkSearchMsgQueue: make(chan *cmn.PeerMessage, msgQueueSize),
		TxsAvailable:       make(chan uint64, 1),
		done:               make(chan struct{}),
		syncHandler:        new(SyncHandler),
		recvSyncChan:       make(chan *psync.SyncNotify, msgQueueSize), //todo 改成专门的接收管道长度

		IsVoteBroadcast:        false,
		timeState:              NewTimeStatistic(),
		consSucceedFuncChain:   make(map[string]func(tdmMsgHander *TDMMsgHandler, blk *types.Block) error),
		statusUpdatedFuncChain: make(map[string]func(tdmMsgHander *TDMMsgHandler) error),
		JustStarted:            true,
		IsContinueConsensus:    true,
	}

	ConsLog = logging.GetLogIns()

	msgHandler.isSyncStatus = new(cmn.MutexBool)
	msgHandler.isSyncStatus.SetFalse()
	msgHandler.cacheSyncBlkCount = make(map[string]*SyncBlkCount)

	msgHandler.doPrevote = msgHandler.defaultDoPrevote

	msgHandler.writeBlock = appMgr.BlockProcess

	msgHandler.ID = msp.GetPeerID()

	digestAddr := msp.AccountPubkeyToAddress()

	msgHandler.digestAddr = util.HexBytes(digestAddr.GetBytes())

	err := msgHandler.updateToState(lastCommitState)
	if err != nil {
		return nil, err
	}

	err = msgHandler.reconstructLastCommit(lastCommitState, nil)
	if err != nil {
		return nil, err
	}

	msgHandler.BaseService = *cmn.NewBaseService("msgHandler", msgHandler)
	msgHandler.heightReq = NewHeightReq()

	msgHandler.syncTimeout = time.Duration(config.Config.BlkTimeout) * time.Second
	msgHandler.syncTimer = time.NewTimer(msgHandler.syncTimeout)
	return msgHandler, nil
}

func (h *TDMMsgHandler) checkTxsAvailable() {
	h.allRoutineExitWg.Add(1)
	defer h.allRoutineExitWg.Done()

	lastHeight := h.Height - 1
	for {
		select {
		case <-txpool.NotifyTx(appComm.HNB):

			if h.Height > lastHeight && h.Step == types.RoundStepNewRound {
				select {
				case h.TxsAvailable <- h.Height:

					h.JustStarted = false
					lastHeight = h.Height
					ConsLog.Infof(LOGTABLE_CONS, "(check txs)txs available, propose h %d", h.Height)
				default:
					ConsLog.Debugf(LOGTABLE_CONS, "(check txs) already deal txs", h.Height)
				}
			}

		case <-h.Quit():
			ConsLog.Infof(LOGTABLE_CONS, "(check txs) consensus service stop, stop send proposal msg")
			return
		}
	}
}

// Reconstruct LastCommit from SeenCommit, which we saved along with the block,
// (which happens even before saving the state)
func (h *TDMMsgHandler) reconstructLastCommit(lastCommitState state.State, blk *bsComm.Block) error {
	if lastCommitState.LastBlockNum == 0 {
		return nil
	}

	seenCommit, err := h.LoadSeenCommit(lastCommitState.LastBlockNum, blk)
	if err != nil {
		return err
	}

	lastPrecommits := types.NewVoteSet(lastCommitState.LastBlockNum, seenCommit.Round(), types.VoteTypePrecommit, lastCommitState.Validators)
	for _, precommit := range seenCommit.Precommits {
		if precommit == nil {
			continue
		}
		added, err := lastPrecommits.AddVote(precommit)
		if !added || err != nil {
			return fmt.Errorf("failed to reconstruct LastCommit: %v", err)
		}
	}

	if lastCommitState.LastBlockNum != 1 && !lastPrecommits.HasTwoThirdsMajority() {
		return errors.New("failed to reconstruct LastCommit: Does not have +2/3 maj")
	}

	h.LastCommit = lastPrecommits
	return nil
}

func (h *TDMMsgHandler) LoadSeenCommit(blkNum uint64, blk *bsComm.Block) (*types.Commit, error) {
	var err error
	if blk == nil {
		blk, err = ledger.GetBlock(blkNum)
		if err != nil {
			return nil, err
		}

		if blk == nil {
			return nil, fmt.Errorf("blk %d nil", blkNum)
		}
	}

	tdmBlk, err := types.Standard2Cons(blk)
	if err != nil {
		return nil, err
	}
	ConsLog.Infof(LOGTABLE_CONS, "CurrentCommit=%v", tdmBlk.CurrentCommit.StringIndented("-"))
	ConsLog.Infof(LOGTABLE_CONS, "LastCommit=%v", tdmBlk.LastCommit.StringIndented("-"))

	return tdmBlk.CurrentCommit, nil
}

// send a msg into the receiveRoutine regarding our own proposal, block part, or vote
func (h *TDMMsgHandler) sendInternalMessage(msg *cmn.PeerMessage) {
	msg.PeerID = p2pNetwork.GetLocatePeerID()

	select {
	case h.InternalMsgQueue <- msg:
	default:
		go func() { h.InternalMsgQueue <- msg }()
	}
}

func (h *TDMMsgHandler) BroadcastMsgToAll(msg *cmn.PeerMessage) {
	select {
	case h.EventMsgQueue <- msg:
	default:
		ConsLog.Infof(LOGTABLE_CONS, "broadCast msg queue is full. Using a go-routine")
		go func() { h.EventMsgQueue <- msg }()

	}
}

func (h *TDMMsgHandler) OnStart() error {
	err := h.timeoutTicker.Start()
	if err != nil {
		fmt.Print("进入TDMMsgHandler OnStart（）的ERROR")
		return err
	}

	go h.DeliverMsg()
	go h.BroadcastMsg()
	h.stopSyncTimer()
	go h.Monitor() // add for monitor block height with other peers
	go h.syncServer()
	go h.recvForkSearch() // add for recv forksearch msgs

	if h.inbgGroup() {
		go h.checkTxsAvailable()

		if h.Round == 0 && types.RoundStepNewHeight == h.Step {
			h.scheduleRound0(h.GetRoundState())
		} else {
			h.setTimeout()
		}
	}
	h.IsContinueConsensus = true

	return nil
}

func (h *TDMMsgHandler) setTimeout() {
	switch h.Step {
	case types.RoundStepPropose:
		timeoutPropose := config.Config.ConsensusConfig.TimeoutPropose
		h.scheduleTimeout(h.AddTimeOut(timeoutPropose), h.Height, h.Round, types.RoundStepPropose)
	case types.RoundStepPrevote:
		timeoutPrevote := config.Config.ConsensusConfig.TimeoutPrevote
		h.scheduleTimeout(h.AddTimeOut(timeoutPrevote), h.Height, h.Round, types.RoundStepPrevote)
	case types.RoundStepPrevoteWait:
		timeoutPrevoteWait := config.Config.ConsensusConfig.TimeoutPrevoteWait
		h.scheduleTimeout(h.AddTimeOut(timeoutPrevoteWait), h.Height, h.Round, types.RoundStepPrevoteWait)
	case types.RoundStepPrecommit:
		timeoutPrecommit := config.Config.ConsensusConfig.TimeoutPrecommit
		h.scheduleTimeout(h.AddTimeOut(timeoutPrecommit), h.Height, h.Round, types.RoundStepPrecommit)
	case types.RoundStepPrecommitWait:
		timeoutPrecommitWait := config.Config.ConsensusConfig.TimeoutPrecommitWait
		h.scheduleTimeout(h.AddTimeOut(timeoutPrecommitWait), h.Height, h.Round, types.RoundStepPrecommitWait)
	case types.RoundStepNewHeight:
		timeoutNewRound := config.Config.ConsensusConfig.TimeoutNewRound
		h.scheduleTimeout(h.AddTimeOut(timeoutNewRound), h.Height, 0, types.RoundStepNewHeight)
	case types.RoundStepNewRound:
		h.dealNewRound()
	}
}

func (h *TDMMsgHandler) dealNewRound() {
	waitForTxs := h.WaitForTxs()
	timeoutWaitFortx := config.Config.ConsensusConfig.TimeoutWaitFortx
	if waitForTxs {
		h.timeState.EndConsumeTime(h)
		if txpool.IsTxsLenZero(appComm.HNB) == false {
			h.enterPropose(h.Height, h.Round)
		} else {
			h.scheduleTimeout(h.AddTimeOut(timeoutWaitFortx), h.Height, h.Round, types.RoundStepNewRound)
		}
	} else if config.Config.ConsensusConfig.CreateEmptyBlocksInterval > 0 {
		h.timeState.EndConsumeTime(h)
		td := time.Duration(config.Config.ConsensusConfig.CreateEmptyBlocksInterval) * time.Millisecond
		h.scheduleTimeout(td, h.Height, h.Round, types.RoundStepNewRound)
	} else {
		h.timeState.EndConsumeTime(h)
		h.enterPropose(h.Height, h.Round)
	}
}

func (h *TDMMsgHandler) inbgGroup() bool {
	for _, val := range h.Validators.Validators {
		if bytes.Equal(val.Address, h.digestAddr) {
			return true
		}
	}

	return false
}

func (h *TDMMsgHandler) isPeerInbgGroup(pubKeyID []byte) bool {
	for _, val := range h.Validators.Validators {
		if bytes.Equal(val.Address, h.digestAddr) {
			return true
		}

	}

	return false
}

func (h *TDMMsgHandler) OnStop() {
	h.timeoutTicker.Stop()
}

func (h *TDMMsgHandler) OnReset() error {
	h.PeerMsgQueue = make(chan *cmn.PeerMessage, msgQueueSize)
	h.InternalMsgQueue = make(chan *cmn.PeerMessage, msgQueueSize)
	h.EventMsgQueue = make(chan *cmn.PeerMessage, msgQueueSize)
	h.TxsAvailable = make(chan uint64, 1)
	h.done = make(chan struct{})

	h.recvSyncChan = make(chan *psync.SyncNotify, msgQueueSize)
	h.isSyncStatus.SetFalse()
	h.stopSyncTimer()
	h.updateRoundStep(h.Round, types.RoundStepNewRound)
	h.timeoutTicker.Reset()

	return nil
}

func (h *TDMMsgHandler) String() string {
	return cmn.Fmt("tdmMsgHandler{tdm cons service}")
}

// new round after start
func (h *TDMMsgHandler) scheduleRound0(rs *types.RoundState) {
	timeoutNewRound := config.Config.TimeoutNewRound
	if h.JustStarted {
		h.scheduleTimeout(h.AddTimeOut(timeoutNewRound), rs.Height, 0, types.RoundStepNewHeight)
	} else {
		h.enterNewRound(rs.Height, 0)
	}
}

func (h *TDMMsgHandler) saveBlock(block *types.Block, s *state.State) (*state.State, error) {
	blk, _ := types.ConsToStandard(block)
	blk.Header.PreviousHash = h.LastCommitState.PreviousHash
	err := appMgr.BlockProcess(blk)
	if err != nil {
		return nil, err
	}

	s.PreviousHash, err = ledger.CalcBlockHash(blk)
	if err != nil {
		return nil, err
	}

	ConsLog.Infof(LOGTABLE_CONS, "WriteBlockSuccess TxLength=%v, BlockNum=%v", len(blk.Txs), block.BlockNum)
	h.timeState.SetTxNum(len(blk.Txs))
	h.timeState.SetHeight(block.BlockNum)
	h.timeState.SetRound(h.Round)

	txpool.DelTxs(appComm.HNB, blk.Txs)
	ConsLog.Infof(LOGTABLE_CONS, "save blk successful")

	return s, nil
}

func (h *TDMMsgHandler) getHeightFromLedger() uint64 {
	height, _ := ledger.GetBlockHeight()
	return height
}

func (h *TDMMsgHandler) LoadLastCommitStateFromBlkAndMem(newBlk *bsComm.Block) (*state.State, error) {
	if newBlk.Header.BlockNum == 0 {
		return nil, errors.New("can not be num 0")
	}

	tdmBlk, err := types.Standard2Cons(newBlk)
	if err != nil {
		return nil, err
	}

	curValidators, err := h.LoadValidators(newBlk)
	if err != nil {
		return nil, err
	}

	lastBlkID := tdmBlk.CurrentCommit.BlockID

	return &state.State{
		LastBlockNum:                tdmBlk.BlockNum,
		LastBlockTime:               tdmBlk.Time,
		LastBlockID:                 lastBlkID,
		LastBlockTotalTx:            tdmBlk.Header.TotalTxs,
		Validators:                  curValidators,
		LastValidators:              h.LastCommitState.Validators,
		LastHeightValidatorsChanged: tdmBlk.LastHeightChanged,
		PrevVRFValue:                tdmBlk.BlkVRFValue,
		PrevVRFProof:                tdmBlk.BlkVRFProof,
	}, nil

}

func (h *TDMMsgHandler) LoadLastCommitStateFromBlk(block *bsComm.Block) (*state.State, error) {
	tdmBlk, err := types.Standard2Cons(block)
	if err != nil {
		return nil, err
	}

	curValidators, err := h.LoadValidators(block)
	if err != nil {
		return nil, err
	}

	var lastValidators *types.ValidatorSet
	if block.Header.BlockNum == 0 {
		lastValidators = types.NewValidatorSet(nil, 0, nil, nil, nil)
	} else {
		prevBlk, err := ledger.GetBlock(block.Header.BlockNum - 1)
		if prevBlk == nil {
			return nil, fmt.Errorf("tdm get last blk %d nil %v", block.Header.BlockNum-1, err)
		}

		if err != nil {
			return nil, err
		}

		lastValidators, err = h.LoadValidators(prevBlk)
		if err != nil {
			return nil, err
		}
	}

	var lastBlkID types.BlockID
	if tdmBlk.BlockNum != 0 {
		lastBlkID = tdmBlk.CurrentCommit.BlockID
	}

	return &state.State{
		LastBlockNum:                tdmBlk.BlockNum,
		LastBlockTime:               tdmBlk.Time,
		LastBlockID:                 lastBlkID,
		LastBlockTotalTx:            tdmBlk.Header.TotalTxs,
		Validators:                  curValidators,
		LastValidators:              lastValidators,
		LastHeightValidatorsChanged: tdmBlk.LastHeightChanged,
		PrevVRFValue:                tdmBlk.BlkVRFValue,
		PrevVRFProof:                tdmBlk.BlkVRFProof,
	}, nil
}

func (h *TDMMsgHandler) AddTimeOut(timeout int) time.Duration {
	var timer int32 = 3
	if h.Round > timer {
		return time.Duration(timeout)*time.Millisecond + time.Duration(timer*1000)*time.Millisecond
	} else {
		return time.Duration(timeout)*time.Millisecond + time.Duration(h.Round*1000)*time.Millisecond
	}
}

func (h *TDMMsgHandler) WaitForTxs() bool {
	return !config.Config.ConsensusConfig.CreateEmptyBlocks
}

func (h *TDMMsgHandler) WaitForTxsNil() bool {
	return !config.Config.CreateEmptyBlocks && config.Config.CreateEmptyBlocksInterval > 0
}

func (h *TDMMsgHandler) Commit(t time.Time) time.Time {
	return t.Add(time.Duration(config.Config.TimeoutCommit) * time.Millisecond)
}

func (h *TDMMsgHandler) LoadLastCommitStateFromCurHeight() (*state.State, error) {
	curH, err := ledger.GetBlockHeight()

	if err != nil {
		return nil, err
	}

	ConsLog.Infof(LOGTABLE_CONS, "(LoadLastCommitStateFromCurHeight) curH=%v", curH)

	blk, err := ledger.GetBlock(curH - 1)
	if err != nil {
		return nil, err
	}

	if blk == nil {
		return nil, fmt.Errorf("blk %d is nil", curH-1)
	}

	lastCommitState, err := h.LoadLastCommitStateFromBlk(blk)
	if err != nil {
		return nil, err
	}

	return lastCommitState, nil
}

// load validator from block
func (h *TDMMsgHandler) LoadValidators(blk *bsComm.Block) (curVal *types.ValidatorSet, err error) {
	tdmBlk, err := types.Standard2Cons(blk)
	valLastHeightChanged := tdmBlk.LastHeightChanged
	if err != nil {
		return nil, err
	}

	curVal = tdmBlk.Validators
	if curVal == nil {
		lastValidatorChangeHeightBlk, err := ledger.GetBlock(valLastHeightChanged)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "tdm get blk %d err %v", valLastHeightChanged, err)
			return nil, err
		}

		if lastValidatorChangeHeightBlk == nil {
			ConsLog.Errorf(LOGTABLE_CONS, "blk %d is nil", valLastHeightChanged)
			return nil, fmt.Errorf(" blk %d is nil", valLastHeightChanged)
		}

		lastTdmBlk, err := types.Standard2Cons(lastValidatorChangeHeightBlk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "tdm get blk %d err %v", valLastHeightChanged, err)
			return nil, err
		}

		if lastTdmBlk.Validators == nil {
			return nil, fmt.Errorf("%d load validators err", valLastHeightChanged)
		}

		curVal = lastTdmBlk.Validators
	}

	return curVal, nil
}

func (h *TDMMsgHandler) GetDigestAddr() util.HexBytes {
	return h.digestAddr
}
