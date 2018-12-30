package txpool

import (
	"HNB/p2pNetwork/message/reqMsg"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	dbComm "github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"math/big"
	"sort"
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

// Note, this method assumes the pool lock is held!
func (pool *TxPool) enqueueTx(hash common.Hash, tx *common.Transaction) (bool, error) {
	// Try to insert the transaction into the future queue
	//from, _ := msp.Sender(pool.signer, tx) // already validate
	from := tx.From
	if pool.queue[from] == nil {
		pool.queue[from] = newTxList(false)
	}

	TXPoolLog.Debugf(LOGTABLE_TXPOOL, "insert queue from:%v hash:%v", from, hash)

	inserted, old := pool.queue[from].Add(tx)
	if !inserted {
		// An older transaction was better, discard this
		return false, ErrReplaceUnderpriced
	}
	// Discard any previous transaction and mark this
	if old != nil {
		pool.all.Remove(old.Hash())
	}
	if pool.all.Get(hash) == nil {
		pool.all.Add(tx)
	}

	return old != nil, nil
}

// journalTx adds the specified transaction to the local disk journal if it is
// deemed to have been sent from a local account.
func (pool *TxPool) journalTx(from common.Address, tx *common.Transaction) {
	// Only journal if it's enabled and the transaction is local
	if pool.journal == nil || !pool.locals.contains(from) {
		return
	}
	if err := pool.journal.insert(tx); err != nil {
		TXPoolLog.Warningf(LOGTABLE_TXPOOL, "Failed to journal local transaction", "err", err)
	}
}

// promoteTx adds a transaction to the pending (processable) list of transactions
// and returns whether it was inserted or an older was better.
//
// Note, this method assumes the pool lock is held!
func (pool *TxPool) promoteTx(addr common.Address, hash common.Hash, tx *common.Transaction) bool {
	// Try to insert the transaction into the pending queue
	if pool.pending[addr] == nil {
		pool.pending[addr] = newTxList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx)
	if !inserted {
		// An older transaction was better, discard this
		pool.all.Remove(hash)
		return false
	}
	// Otherwise discard any previous transaction and mark this
	if old != nil {
		pool.all.Remove(old.Hash())
	}
	// Failsafe to work around direct pending inserts (tests)
	if pool.all.Get(hash) == nil {
		pool.all.Add(tx)
	}
	// Set the potentially new pending nonce and notify any subsystems of the new tx
	pool.beats[addr] = time.Now()
	pool.pendingState.SetNonce(addr, tx.Nonce()+1)

	return true
}

// AddLocal enqueues a single transaction into the pool if it is valid, marking
// the sender as a local one in the mean time, ensuring it goes around the local
// pricing constraints.
func (pool *TxPool) AddLocal(tx *common.Transaction) error {
	return pool.addTx(tx, !pool.config.NoLocals)
}

// AddRemote enqueues a single transaction into the pool if it is valid. If the
// sender is not among the locally tracked ones, full pricing constraints will
// apply.
func (pool *TxPool) AddRemote(tx *common.Transaction) error {
	return pool.addTx(tx, false)
}

// AddLocals enqueues a batch of transactions into the pool if they are valid,
// marking the senders as a local ones in the mean time, ensuring they go around
// the local pricing constraints.
func (pool *TxPool) AddLocals(txs []*common.Transaction) []error {
	return pool.addTxs(txs, !pool.config.NoLocals)
}

// AddRemotes enqueues a batch of transactions into the pool if they are valid.
// If the senders are not among the locally tracked ones, full pricing constraints
// will apply.
func (pool *TxPool) AddRemotes(txs []*common.Transaction) []error {
	return pool.addTxs(txs, false)
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *TxPool) addTx(tx *common.Transaction, local bool) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to inject the transaction and update any state
	_, err := pool.add(tx, local)
	if err != nil {
		return err
	}

	pool.promoteExecutables([]common.Address{tx.FromAddress()})
	return nil
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxPool) addTxs(txs []*common.Transaction, local bool) []error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.addTxsLocked(txs, local)
}

// addTxsLocked attempts to queue a batch of transactions if they are valid,
// whilst assuming the transaction pool lock is already held.
func (pool *TxPool) addTxsLocked(txs []*common.Transaction, local bool) []error {
	// Add the batch of transaction, tracking the accepted ones
	dirty := make(map[common.Address]struct{})
	errs := make([]error, len(txs))

	for i, tx := range txs {
		var replace bool
		if replace, errs[i] = pool.add(tx, local); errs[i] == nil && !replace {
			//from, _ := msp.Sender(pool.signer, tx) // already validated
			from := tx.From
			dirty[from] = struct{}{}
		}
	}
	// Only reprocess the internal state if something was actually added
	if len(dirty) > 0 {
		addrs := make([]common.Address, 0, len(dirty))
		for addr := range dirty {
			addrs = append(addrs, addr)
		}
		pool.promoteExecutables(addrs)
	}
	return errs
}

// Status returns the status (unknown/pending/queued) of a batch of transactions
// identified by their hashes.
func (pool *TxPool) Status(hashes []common.Hash) []TxStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]TxStatus, len(hashes))
	for i, hash := range hashes {
		if tx := pool.all.Get(hash); tx != nil {
			//from, _ := msp.Sender(pool.signer, tx) // already validated
			from := tx.From
			if pool.pending[from] != nil && pool.pending[from].txs.items[tx.Nonce()] != nil {
				status[i] = TxStatusPending
			} else {
				status[i] = TxStatusQueued
			}
		}
	}
	return status
}

// Get returns a transaction if it is contained in the pool
// and nil otherwise.
func (pool *TxPool) Get(hash common.Hash) *common.Transaction {
	return pool.all.Get(hash)
}

// removeTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
func (pool *TxPool) removeTx(hash common.Hash, outofbound bool) {
	// Fetch the transaction we wish to delete
	tx := pool.all.Get(hash)
	if tx == nil {
		return
	}
	//addr, _ := msp.Sender(pool.signer, tx) // already validated during insertion
	addr := tx.From
	// Remove it from the list of known transactions
	pool.all.Remove(hash)

	// Remove the transaction from the pending lists and reset the account nonce
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
			// If no more pending transactions are left, remove the list
			if pending.Empty() {
				delete(pool.pending, addr)
				delete(pool.beats, addr)
			}
			// Postpone any invalidated transactions
			for _, tx := range invalids {
				pool.enqueueTx(tx.Hash(), tx)
			}
			// Update the account nonce if needed
			if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
				pool.pendingState.SetNonce(addr, nonce)
			}
			return
		}
	}
	// Transaction is in the future queue
	if future := pool.queue[addr]; future != nil {
		future.Remove(tx)
		if future.Empty() {
			delete(pool.queue, addr)
		}
	}
}

// promoteExecutables moves transactions that have become processable from the
// future queue to the set of pending transactions. During this process, all
// invalidated transactions (low nonce, low balance) are deleted.
func (pool *TxPool) promoteExecutables(accounts []common.Address) {
	// Track the promoted transactions to broadcast them at once
	var promoted []*common.Transaction

	// Gather all the accounts potentially needing updates
	if accounts == nil {
		accounts = make([]common.Address, 0, len(pool.queue))
		for addr := range pool.queue {
			accounts = append(accounts, addr)
		}
	}
	// Iterate over all accounts and promote any executable transactions
	for _, addr := range accounts {
		list := pool.queue[addr]
		if list == nil {
			TXPoolLog.Debugf(LOGTABLE_TXPOOL, "accounts %v list = nil", addr)
			continue // Just in case someone calls with a non existing account
		}
		// Drop all transactions that are deemed too old (low nonce)
		nonce, _ := ledger.GetNonce(addr)
		for _, tx := range list.Forward(nonce) {
			hash := tx.Hash()
			TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Removed old queued transaction", "hash", hash)
			pool.all.Remove(hash)
		}
		// Drop all transactions that are too costly (low balance or out of gas)
		//drops, _ := list.Filter(pool.currentState.GetBalance(addr))
		//for _, tx := range drops {
		//	hash := tx.Hash()
		//	TXPoolLog.Debugf(LOGTABLE_TXPOOL,"Removed unpayable queued transaction", "hash", hash)
		//	pool.all.Remove(hash)
		//}

		TXPoolLog.Debugf(LOGTABLE_TXPOOL, "pending nonce:%v", pool.pendingState.GetNonce(addr))

		// Gather all executable transactions and promote them
		for _, tx := range list.Ready(pool.pendingState.GetNonce(addr)) {
			hash := tx.Hash()
			TXPoolLog.Debugf(LOGTABLE_TXPOOL, "tx hash %v valid", hash)
			if pool.promoteTx(addr, hash, tx) {
				TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Promoting queued transaction", "hash", hash)
				promoted = append(promoted, tx)
			}
		}
		// Drop all transactions over the allowed limit
		if !pool.locals.contains(addr) {
			for _, tx := range list.Cap(int(pool.config.AccountQueue)) {
				hash := tx.Hash()
				pool.all.Remove(hash)
				TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Removed cap-exceeding queued transaction", "hash", hash)
			}
		}
		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.queue, addr)
		}
	}
	// Notify subsystem for new promoted transactions.
	//if len(promoted) > 0 {
	//go pool.txFeed.Send(NewTxsEvent{promoted})
	//}

	pool.Notify()

	// If the pending limit is overflown, start equalizing allowances
	pending := uint64(0)
	for _, list := range pool.pending {
		pending += uint64(list.Len())
	}
	if pending > pool.config.GlobalSlots {
		//pendingBeforeCap := pending
		// Assemble a spam order to penalize large transactors first
		spammers := prque.New(nil)
		for addr, list := range pool.pending {
			// Only evict transactions from high rollers
			if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
				spammers.Push(addr, int64(list.Len()))
			}
		}
		// Gradually drop transactions from offenders
		offenders := []common.Address{}
		for pending > pool.config.GlobalSlots && !spammers.Empty() {
			// Retrieve the next offender if not local address
			offender, _ := spammers.Pop()
			offenders = append(offenders, offender.(common.Address))

			// Equalize balances until all the same or below threshold
			if len(offenders) > 1 {
				// Calculate the equalization threshold for all current offenders
				threshold := pool.pending[offender.(common.Address)].Len()

				// Iteratively reduce all offenders until below limit or threshold reached
				for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
					for i := 0; i < len(offenders)-1; i++ {
						list := pool.pending[offenders[i]]
						for _, tx := range list.Cap(list.Len() - 1) {
							// Drop the transaction from the global pools too
							hash := tx.Hash()
							pool.all.Remove(hash)

							// Update the account nonce to the dropped transaction
							if nonce := tx.Nonce(); pool.pendingState.GetNonce(offenders[i]) > nonce {
								pool.pendingState.SetNonce(offenders[i], nonce)
							}
							TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Removed fairness-exceeding pending transaction", "hash", hash)
						}
						pending--
					}
				}
			}
		}
		// If still above threshold, reduce to limit or min allowance
		if pending > pool.config.GlobalSlots && len(offenders) > 0 {
			for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
				for _, addr := range offenders {
					list := pool.pending[addr]
					for _, tx := range list.Cap(list.Len() - 1) {
						// Drop the transaction from the global pools too
						hash := tx.Hash()
						pool.all.Remove(hash)

						// Update the account nonce to the dropped transaction
						if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
							pool.pendingState.SetNonce(addr, nonce)
						}
						TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Removed fairness-exceeding pending transaction", "hash", hash)
					}
					pending--
				}
			}
		}
	}
	// If we've queued more transactions than the hard limit, drop oldest ones
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued > pool.config.GlobalQueue {
		// Sort all accounts with queued transactions by heartbeat
		addresses := make(addressesByHeartbeat, 0, len(pool.queue))
		for addr := range pool.queue {
			if !pool.locals.contains(addr) { // don't drop locals
				addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
			}
		}
		sort.Sort(addresses)

		// Drop transactions until the total is below the limit or only locals remain
		for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
			addr := addresses[len(addresses)-1]
			list := pool.queue[addr.address]

			addresses = addresses[:len(addresses)-1]

			// Drop all transactions if they are less than the overflow
			if size := uint64(list.Len()); size <= drop {
				for _, tx := range list.Flatten() {
					pool.removeTx(tx.Hash(), true)
				}
				drop -= size
				continue
			}
			// Otherwise drop only last few transactions
			txs := list.Flatten()
			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
				pool.removeTx(txs[i].Hash(), true)
				drop--
			}
		}
	}
}

// demoteUnexecutables removes invalid and processed transactions from the pools
// executable/pending queue and any subsequent transactions that become unexecutable
// are moved back into the future queue.
func (pool *TxPool) demoteUnexecutables() {
	// Iterate over all accounts and demote any non-executable transactions
	for addr, list := range pool.pending {

		nonce, _ := ledger.GetNonce(addr)
		TXPoolLog.Debugf(LOGTABLE_TXPOOL, "ledger nonce %v", nonce)
		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range list.Forward(nonce) {
			hash := tx.Hash()
			TXPoolLog.Debugf(LOGTABLE_TXPOOL, "Removed old pending transaction", "hash", hash)
			pool.all.Remove(hash)
		}

		// If there's a gap in front, alert (should never happen) and postpone all transactions
		if list.Len() > 0 && list.txs.Get(nonce) == nil {
			for _, tx := range list.Cap(0) {
				hash := tx.Hash()
				TXPoolLog.Errorf(LOGTABLE_TXPOOL, "Demoting invalidated transaction", "hash", hash)
				pool.enqueueTx(hash, tx)
			}
		}
		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.pending, addr)
			delete(pool.beats, addr)
		}
	}
}

func (pool *TxPool) Notify() {
	select {
	case pool.notify <- struct{}{}:
	default:
	}
}

func (pool *TxPool) GetNotify() chan struct{} {
	return pool.notify
}

func (pool *TxPool) handlerTx() {

	for {
		select {
		case msgTx := <-pool.recvTx:
			TXPoolLog.Infof("recv tx msg:%v", string(msgTx.recvTx))
			tx := common.Transaction{}
			err := json.Unmarshal(msgTx.recvTx, &tx)
			if err != nil {
				TXPoolLog.Warning(LOGTABLE_TXPOOL, err.Error())
				return
			}

			if tx.Type == "" {
				TXPoolLog.Infof(LOGTABLE_TXPOOL, "txid =%s chainID = nil", fmt.Sprintf("%x", tx.Txid))
				continue
			}
			if msgTx.local == true {
				m := reqMsg.NewTxMsg(msgTx.recvTx)
				p2pNetwork.Xmit(m, false)

				pool.AddLocal(&tx)
			} else {
				pool.AddRemote(&tx)
			}

			infoStr := fmt.Sprintf("recv tx  %v", tx)
			TXPoolLog.Debugf(LOGTABLE_TXPOOL, infoStr)
		}
	}

}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addressesByHeartbeat []addressByHeartbeat

func (a addressesByHeartbeat) Len() int           { return len(a) }
func (a addressesByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addressesByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// accountSet is simply a set of addresses to check for existence, and a signer
// capable of deriving addresses from transactions.
type accountSet struct {
	accounts map[common.Address]struct{}
	signer   msp.Signer
	cache    *[]common.Address
}

// newAccountSet creates a new address set with an associated signer for sender
// derivations.
func newAccountSet(signer msp.Signer) *accountSet {
	return &accountSet{
		accounts: make(map[common.Address]struct{}),
		signer:   signer,
	}
}

// contains checks if a given address is contained within the set.
func (as *accountSet) contains(addr common.Address) bool {
	_, exist := as.accounts[addr]
	return exist
}

// containsTx checks if the sender of a given tx is within the set. If the sender
// cannot be derived, this method returns false.
func (as *accountSet) containsTx(tx *common.Transaction) bool {
	//if addr, err := msp.Sender(as.signer, tx); err == nil {
	////	if addr, err := msp.Sender(as.signer, tx); err == nil {
	//	return as.contains(addr)
	//}
	//return false
	return as.contains(tx.From)

}

// add inserts a new address into the set to track.
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
	as.cache = nil
}

// flatten returns the list of addresses within this set, also caching it for later
// reuse. The returned slice should not be changed!
func (as *accountSet) flatten() []common.Address {
	if as.cache == nil {
		accounts := make([]common.Address, 0, len(as.accounts))
		for account := range as.accounts {
			accounts = append(accounts, account)
		}
		as.cache = &accounts
	}
	return *as.cache
}

type txLookup struct {
	all  map[common.Hash]*common.Transaction
	lock sync.RWMutex
}

// newTxLookup returns a new txLookup structure.
func newTxLookup() *txLookup {
	return &txLookup{
		all: make(map[common.Hash]*common.Transaction),
	}
}

// Range calls f on each key and value present in the map.
func (t *txLookup) Range(f func(hash common.Hash, tx *common.Transaction) bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for key, value := range t.all {
		if !f(key, value) {
			break
		}
	}
}

// Get returns a transaction if it exists in the lookup, or nil if not found.
func (t *txLookup) Get(hash common.Hash) *common.Transaction {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.all[hash]
}

// Count returns the current number of items in the lookup.
func (t *txLookup) Count() int {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return len(t.all)
}

// Add adds a transaction to the lookup.
func (t *txLookup) Add(tx *common.Transaction) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.all[tx.Hash()] = tx
}

// Remove removes a transaction from the lookup.
func (t *txLookup) Remove(hash common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()

	delete(t.all, hash)
}
