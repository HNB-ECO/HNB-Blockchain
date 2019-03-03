package leveldbImpl

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelDBStore struct {
	db    *leveldb.DB
	batch *leveldb.Batch
}

const BITSPERKEY = 10

func NewLevelDBStore(file string) (*LevelDBStore, error) {

	o := opt.Options{
		NoSync: false,
		Filter: filter.NewBloomFilter(BITSPERKEY),
	}

	db, err := leveldb.OpenFile(file, &o)

	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}

	if err != nil {
		return nil, err
	}

	return &LevelDBStore{
		db:    db,
		batch: nil,
	}, nil
}

func (self *LevelDBStore) Put(key []byte, value []byte) error {
	return self.db.Put(key, value, nil)
}

func (self *LevelDBStore) Get(key []byte) ([]byte, error) {
	dat, err := self.db.Get(key, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return dat, nil
}

func (self *LevelDBStore) Has(key []byte) (bool, error) {
	return self.db.Has(key, nil)
}

func (self *LevelDBStore) Delete(key []byte) error {
	return self.db.Delete(key, nil)
}

func (self *LevelDBStore) NewBatch() {
	self.batch = new(leveldb.Batch)
}

func (self *LevelDBStore) BatchPut(key []byte, value []byte) {
	self.batch.Put(key, value)
}

func (self *LevelDBStore) BatchDelete(key []byte) {
	self.batch.Delete(key)
}

func (self *LevelDBStore) BatchCommit() error {
	err := self.db.Write(self.batch, nil)
	if err != nil {
		return err
	}
	self.batch = nil
	return nil
}

func (self *LevelDBStore) Close() error {
	err := self.db.Close()
	return err
}

func (self *LevelDBStore) NewIterator(prefix []byte) common.Iterator {

	iter := self.db.NewIterator(util.BytesPrefix(prefix), nil)

	return &Iterator{
		iter: iter,
	}
}
