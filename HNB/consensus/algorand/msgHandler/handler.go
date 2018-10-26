package msgHandler

const CHAINID = "hgs"

var msgQueueSize = 1000
var (
	ErrInvalidProposalSignature = errors.New("Error invalid proposal signature")
	ErrInvalidProposalPOLRound  = errors.New("Error invalid proposal POL round")
	ErrAddingVote               = errors.New("Error adding vote")
	ErrVoteHeightMismatch       = errors.New("Error vote height mismatch")
)

func NewTDMMsgHandler(lastCommitState state.State) (*TDMMsgHandler, error) {
	msgHandler := &TDMMsgHandler{
		timeoutTicker:       NewTimeoutTicker(),
		PeerMsgQueue:        make(chan *cmn.PeerMessage, msgQueueSize),
		InternalMsgQueue:    make(chan *cmn.PeerMessage, msgQueueSize),
		EventMsgQueue:       make(chan *cmn.PeerMessage, msgQueueSize),
		SyncMsgQueue:        make(chan *cmn.PeerMessage, msgQueueSize),
		TxsAvailable:        make(chan uint64, 1),
		done:                make(chan struct{}),
		syncHandler:         new(SyncHandler),
		recvSyncChan:        make(chan *psync.SyncNotify, msgQueueSize), //todo 改成专门的接收管道长度

		IsVoteBroadcast:     false,
		timeState:           NewTimeStatistic(),
	}

	ConsLog = logging.GetLogIns()
	msgHandler.isSyncStatus = new(cmn.MutexBool)
	msgHandler.isSyncStatus.SetFalse()
	msgHandler.cacheSyncBlkCount = make(map[string] *SyncBlkCount)

	msgHandler.doPrevote = msgHandler.defaultDoPrevote

	msgHandler.writeBlock = appMgr.BlockProcess

	msgHandler.ID = msp.GetPeerID()

	pubKeyStr := msp.GetPeerPubStr()
	digestAddr, err := msp.Hash256(msp.GetPubBytesFromStr(pubKeyStr))
	if err != nil {
		return nil, err
	}
	msgHandler.digestAddr = cmn.HexBytes(digestAddr)


	err = msgHandler.updateToState(lastCommitState)
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
		case <-txpool.NotifyTx(txpool.HGS):
			if h.Height > lastHeight {
				h.TxsAvailable <- h.Height

				lastHeight = h.Height
				ConsLog.Infof(LOGTABLE_CONS, "(check txs)txs available, propose h %d", h.Height)
			}

		case <-h.Quit():
			ConsLog.Infof(LOGTABLE_CONS, "(check txs) consensus service stop, stop send proposal msg")
			return
		}
	}
}

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
		return err
	}
	go h.DeliverMsg()
	go h.BroadcastMsg()
	go h.checkTxsAvailable()
	h.stopSyncTimer()
	go h.Monitor() // add for monitor block height with other peers
	go h.syncServer()
	if h.inbgGroup() {
		if h.Round == 0 && types.RoundStepNewHeight == h.Step {
			h.scheduleRound0(h.GetRoundState(), true)
		} else {
			h.setTimeout()
		}
	}

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
		if txpool.TxsLen(txpool.HGS) > 0 {
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
		//ConsLog.Infof(LOGTABLE_CONS, "inbgGroup %s %s", val.Address, h.digestAddr)
		if bytes.Equal(val.Address, h.digestAddr) {
			return true
		}
	}

	return false
}
func (h *TDMMsgHandler) isPeerInbgGroup(pubKeyID []byte) bool {
334         for _, val := range h.Validators.Validators {
335                 if bytes.Equal(val.Address, h.digestAddr) {
336                         return true
337                 }
338 
339         }
340 
341         return false
342 }
343 
348 func (h *TDMMsgHandler) OnStop() { 
350         h.timeoutTicker.Stop()
351 }

354 func (h *TDMMsgHandler) OnReset() error {
356         h.PeerMsgQueue = make(chan *cmn.PeerMessage, msgQueueSize)
357         h.InternalMsgQueue = make(chan *cmn.PeerMessage, msgQueueSize)
358         h.EventMsgQueue = make(chan *cmn.PeerMessage, msgQueueSize)
359         h.TxsAvailable = make(chan uint64, 1)
360         h.done = make(chan struct{})
361         
362         h.recvSyncChan = make(chan *psync.SyncNotify, msgQueueSize)
363 
365         h.isSyncStatus.SetFalse()
366         h.stopSyncTimer()
369         h.updateRoundStep(h.Round, types.RoundStepNewRound)
371         h.timeoutTicker.Reset()
372 
373         return nil
374 }
375 
376 func (h *TDMMsgHandler) String() string {
377         return cmn.Fmt("tdmMsgHandler{tdm cons service}")
378 }  
