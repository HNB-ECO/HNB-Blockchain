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

// send a msg into the receiveRoutine regarding our own proposal, block part, or vote
func (h *TDMMsgHandler) sendInternalMessage(msg *cmn.PeerMessage) {
//	ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) (sendInternalMessage) Type=%v,Timestamp=%v", h.Height, h.Round, msg.Msg.Type, msg.Msg.Timestamp)

	msg.PeerID = p2pNetwork.GetLocatePeerID()

	select {
	case h.InternalMsgQueue <- msg:
	default:
//		ConsLog.Infof(LOGTABLE_CONS, "Internal msg queue is full. Using a go-routine")
		go func() { h.InternalMsgQueue <- msg }()
	}
}

func (h *TDMMsgHandler) BroadcastMsgToAll(msg *cmn.PeerMessage) {
	//ConsLog.Infof(LOGTABLE_CONS, "#(%v-%v) (BroadcastMsgToAll) Type=%v,Timestamp=%v", h.Height, h.Round, msg.Msg.Type, msg.Msg.Timestamp)
	select {
	case h.EventMsgQueue <- msg:
	default:
		ConsLog.Infof(LOGTABLE_CONS, "broadCast msg queue is full. Using a go-routine")
		go func() { h.EventMsgQueue <- msg }()

	}
}
