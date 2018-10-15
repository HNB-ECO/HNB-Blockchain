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
