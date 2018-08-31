package common

type KVStore interface {
	Put(key []byte, value []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	Delete(key []byte) error
	NewBatch()
	BatchPut(key []byte, value []byte)
	BatchDelete(key []byte)
	BatchCommit() error
	Close() error
	NewIterator(prefix []byte) Iterator
}


type Iterator interface {
	Next() bool
	Prev() bool
	First() bool
	Last() bool
	Seek(key []byte) bool
	Key() []byte
	Value() []byte
//	Close()
	Release()             //Close iterator
}


