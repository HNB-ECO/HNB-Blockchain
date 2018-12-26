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
type RecvTxStruct struct {
	recvTx []byte
	local  bool
}

// NewTxPool creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func NewTxPool(config TxPoolConfig) *TxPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()
	chainID := new(big.Int)
	chainID.SetBytes([]byte(common.HGS))

	// Create the transaction pool with its initial settings
	pool := &TxPool{
		config:     config,
		signer:     msp.NewHNBSigner(chainID),
		pending:    make(map[common.Address]*txList),
		queue:      make(map[common.Address]*txList),
		beats:      make(map[common.Address]time.Time),
		all:        newTxLookup(),
		recvBlkTxs: make(chan common.Transactions),
		notify:     make(chan struct{}, 10),
		recvTx:     make(chan *RecvTxStruct, 10000),
	}
	pool.locals = newAccountSet(pool.signer)
	for _, addr := range config.Locals {
		TXPoolLog.Infof(LOGTABLE_TXPOOL, "Setting new local account", "address", addr)
		pool.locals.add(addr)
	}

	pool.reset(nil)

	// If local transactions and journaling is enabled, load from disk
	if !config.NoLocals && config.Journal != "" {
		pool.journal = newTxJournal(config.Journal)

		if err := pool.journal.load(pool.AddLocals); err != nil {
			TXPoolLog.Warningf(LOGTABLE_TXPOOL, "Failed to load transaction journal", "err", err)
		}
		if err := pool.journal.rotate(pool.local()); err != nil {
			TXPoolLog.Warningf(LOGTABLE_TXPOOL, "Failed to rotate transaction journal", "err", err)
		}
	}

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	go pool.handlerTx()
	return pool
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *TxPool) loop() {
	defer pool.wg.Done()

	// Start the stats reporting and transaction eviction tickers
	var prevPending, prevQueued int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

	// Track the previous head headers for transaction reorgs

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case txs := <-pool.recvBlkTxs:

			if txs != nil {
				pool.mu.Lock()
				TXPoolLog.Debugf(LOGTABLE_TXPOOL, "del tx length %v", len(txs))
				pool.reset(txs)
				pool.mu.Unlock()
			}

		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.stats()
			pool.mu.RUnlock()

			if pending != prevPending || queued != prevQueued {
				TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Transaction pool status report", "executable", pending, "queued", queued)
				prevPending, prevQueued = pending, queued
			}

			// Handle inactive account transaction eviction
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
				// Skip local transactions from the eviction mechanism
				if pool.locals.contains(addr) {
					continue
				}
				// Any non-locals old enough should be removed
				if time.Since(pool.beats[addr]) > pool.config.Lifetime {
					for _, tx := range pool.queue[addr].Flatten() {
						pool.removeTx(tx.Hash(), true)
					}
				}
			}
			pool.mu.Unlock()

			// Handle local transaction journal rotation
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.local()); err != nil {
					TXPoolLog.Warningf(LOGTABLE_TXPOOL, "Failed to rotate local tx journal", "err", err)
				}
				pool.mu.Unlock()
			}
		}
	}
}
