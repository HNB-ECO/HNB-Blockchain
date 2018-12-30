package txpool

import (
	"errors"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"math/big"
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

func (pool *TxPool) lockedReset() {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.reset(nil)
}

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
func (pool *TxPool) reset(reinject common.Transactions) {
	// If we're reorging an old state, reinject all dropped transactions

	if pool.pendingState == nil {
		pool.pendingState = NewPendingNonce()
	}

	pool.addTxsLocked(reinject, false)

	// validate the pool of pending transactions, this will remove
	// any transactions that have been included in the block or
	// have been invalidated because of another transaction (e.g.
	// higher gas price)
	pool.demoteUnexecutables()

	// Update all accounts to the latest known pending nonce
	for addr, list := range pool.pending {
		txs := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
		pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	}
	// Check the queue and move transactions over to the pending if possible
	// or remove those that have become invalid
	pool.promoteExecutables(nil)
}

// Stop terminates the transaction pool.
func (pool *TxPool) Stop() {

	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	TXPoolLog.Info(LOGTABLE_TXPOOL, "Transaction pool stopped")
}

// State returns the virtual managed state of the transaction pool.
func (pool *TxPool) State() *PendingCache {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) Stats() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

// number of queued (non-executable) transactions.
func (pool *TxPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}

// Content retrieves the data content of the transaction pool, returning all the
// pending as well as queued transactions, grouped by account and sorted by nonce.
func (pool *TxPool) Content() (map[common.Address]common.Transactions, map[common.Address]common.Transactions) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]common.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	queued := make(map[common.Address]common.Transactions)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return pending, queued
}

// Pending retrieves all currently processable transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) Pending() (map[common.Address]common.Transactions, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]common.Transactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	return pending, nil
}

// Locals retrieves the accounts currently considered local by the pool.
func (pool *TxPool) Locals() []common.Address {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.locals.flatten()
}

// local retrieves all currently known local transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) local() map[common.Address]common.Transactions {
	txs := make(map[common.Address]common.Transactions)
	for addr := range pool.locals.accounts {
		if pending := pool.pending[addr]; pending != nil {
			txs[addr] = append(txs[addr], pending.Flatten()...)
		}
		if queued := pool.queue[addr]; queued != nil {
			txs[addr] = append(txs[addr], queued.Flatten()...)
		}
	}
	return txs
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *TxPool) validateTx(tx *common.Transaction, local bool) error {
	//TODO 对交易大小限制

	// Make sure the transaction is signed properly
	//from, err := msp.Sender(pool.signer, tx)
	from := tx.From
	//if err != nil {
	//	TXPoolLog.Errorf(LOGTABLE_TXPOOL, "invalid sender err:%v", err.Error())
	//	return ErrInvalidSender
	//}
	// Drop non-local transactions under our own minimal accepted gas price
	local = local || pool.locals.contains(from) // account may be local even if the transaction arrived from the network
	//如果 交易地址不是本地的，需要计算以下gas是否满足最低要求
	//如果 交易地址是本地的 不需要计算gas是否满足最低要求都要打包

	// Ensure the transaction adheres to nonce ordering
	nonce, _ := ledger.GetNonce(from)

	if nonce > tx.Nonce() {
		return ErrNonceTooLow
	}

	return nil
}

// add validates a transaction and inserts it into the non-executable queue for
// later pending promotion and execution. If the transaction is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
//
// If a newly added transaction is marked as local, its sending account will be
// whitelisted, preventing any associated transaction from being dropped out of
// the pool due to pricing constraints.F
func (pool *TxPool) add(tx *common.Transaction, local bool) (bool, error) {
	// If the transaction is already known, discard it
	hash := tx.Hash()
	if pool.all.Get(hash) != nil {
		TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Discarding already known transaction", "hash", hash)
		return false, fmt.Errorf("known transaction: %x", hash)
	}
	// If the transaction fails basic validation, discard it
	if err := pool.validateTx(tx, local); err != nil {
		TXPoolLog.Errorf(LOGTABLE_TXPOOL, "Discarding invalid transaction hash:%v err:%v",
			util.ByteToHex(hash.GetBytes()), err.Error())
		return false, err
	}
	// If the transaction pool is full, discard underpriced transactions
	if uint64(pool.all.Count()) >= pool.config.GlobalSlots+pool.config.GlobalQueue {
		return false, ErrTxQueueFull
	}
	// If the transaction is replacing an already pending one, do directly
	from := tx.FromAddress()
	if list := pool.pending[from]; list != nil && list.Overlaps(tx) {
		return false, nil
	}
	// New transaction isn't replacing a pending one, push into queue
	replace, err := pool.enqueueTx(hash, tx)
	if err != nil {
		return false, err
	}
	// Mark local addresses and journal local transactions
	if local {
		if !pool.locals.contains(from) {
			TXPoolLog.Infof(LOGTABLE_TXPOOL, "Setting new local account", "address", from)
			pool.locals.add(from)
		}
	}
	pool.journalTx(from, tx)

	TXPoolLog.Infof(LOGTABLE_TXPOOL, "Pooled new future transaction", "hash", hash, "from", from)
	return replace, nil
}
