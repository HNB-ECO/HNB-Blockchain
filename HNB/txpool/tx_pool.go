package txpool

import (
	"errors"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"sync"
	"time"
)

var (
	ErrInvalidSender      = errors.New("invalid sender")
	ErrNonceTooLow        = errors.New("nonce too low")
	ErrUnderpriced        = errors.New("transaction underpriced")
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")
	ErrInsufficientFunds  = errors.New("insufficient funds for gas * price + value")
	ErrIntrinsicGas       = errors.New("intrinsic gas too low")
	ErrGasLimit           = errors.New("exceeds block gas limit")
	ErrNegativeValue      = errors.New("negative value")
	ErrOversizedData      = errors.New("oversized data")
	ErrTxQueueFull        = errors.New("queue full")
)

var (
	evictionInterval    = time.Minute     // Time interval to check for evictable transactions
	statsReportInterval = 8 * time.Second // Time interval to report transaction pool stats
)

type TxStatus uint

const (
	TxStatusUnknown TxStatus = iota
	TxStatusQueued
	TxStatusPending
	TxStatusIncluded
)

type TxPoolConfig struct {
	Locals    []common.Address // Addresses that should be treated by default as local
	Journal   string           // Journal of local transactions to survive node restarts
	Rejournal time.Duration    // Time interval to regenerate the local transaction journal
	NoLocals  bool

	AccountSlots uint64 // Number of executable transaction slots guaranteed per account
	GlobalSlots  uint64 // Maximum number of executable transaction slots for all accounts
	AccountQueue uint64 // Maximum number of non-executable transaction slots permitted per account
	GlobalQueue  uint64 // Maximum number of non-executable transaction slots for all accounts

	Lifetime time.Duration // Maximum amount of time non-executable transaction are queued
}

var DefaultTxPoolConfig = TxPoolConfig{

	Rejournal: time.Hour,

	AccountSlots: 16,
	GlobalSlots:  4096,
	AccountQueue: 64,
	GlobalQueue:  1024,

	Lifetime: 3 * time.Hour,
}

func (config *TxPoolConfig) sanitize() TxPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		TXPoolLog.Warningf(LOGTABLE_TXPOOL, "Sanitizing invalid txpool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	return conf
}

type TxPool struct {
	config       TxPoolConfig
	signer       msp.Signer
	mu           sync.RWMutex
	db           dbComm.KVStore
	pendingState *PendingCache // Pending state tracking virtual nonces
	recvBlkTxs   chan common.Transactions
	notify       chan struct{}
	recvTx       chan *RecvTxStruct

	locals  *accountSet // Set of local transaction to exempt from eviction rules
	journal *txJournal  // Journal of local transaction to back up to disk

	pending map[common.Address]*txList   // All currently processable transactions
	queue   map[common.Address]*txList   // Queued but non-processable transactions
	beats   map[common.Address]time.Time // Last heartbeat from each known account
	all     *txLookup                    // All transactions to allow lookups

	wg sync.WaitGroup // for shutdown sync
}
