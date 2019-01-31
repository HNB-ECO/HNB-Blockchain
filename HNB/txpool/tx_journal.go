package txpool

import (
	"HNB/common"
	"errors"
	"io"
)

// errNoActiveJournal is returned if a transaction is attempted to be inserted
// into the journal, but no such file is currently open.
var errNoActiveJournal = errors.New("no active journal")

// devNull is a WriteCloser that just discards anything written into it. Its
// goal is to allow the transaction journal to write into a fake journal when
// loading transactions on startup without printing warnings due to no file
// being read for write.
type devNull struct{}

func (*devNull) Write(p []byte) (n int, err error) { return len(p), nil }
func (*devNull) Close() error                      { return nil }

// txJournal is a rotating log of transactions with the aim of storing locally
// created transactions to allow non-executed ones to survive node restarts.
type txJournal struct {
	path   string         // Filesystem path to store the transactions at
	writer io.WriteCloser // Output stream to write new transactions into
}

// newTxJournal creates a new transaction journal to
func newTxJournal(path string) *txJournal {
	return &txJournal{
		path: path,
	}
}

// load parses a transaction journal dump from disk, loading its contents into
// the specified pool.
func (journal *txJournal) load(add func([]*common.Transaction) []error) error {
	return nil
}

// insert adds the specified transaction to the local disk journal.
func (journal *txJournal) insert(tx *common.Transaction) error {
	return nil
}

// rotate regenerates the transaction journal based on the current contents of
// the transaction pool.
func (journal *txJournal) rotate(all map[common.Address]common.Transactions) error {
	return nil
}

// close flushes the transaction journal contents to disk and closes the file.
func (journal *txJournal) close() error {
	return nil
}
