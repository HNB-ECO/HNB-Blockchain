package sw

import (
	"errors"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
)

func NewDummyKeyStore() bccsp.KeyStore {
	return &dummyKeyStore{}
}

type dummyKeyStore struct {
}

func (ks *dummyKeyStore) ReadOnly() bool {
	return true
}

func (ks *dummyKeyStore) GetKey(ski []byte) (k bccsp.Key, err error) {
	return nil, errors.New("Key not found. This is a dummy KeyStore")
}

func (ks *dummyKeyStore) StoreKey(k bccsp.Key) (err error) {
	return errors.New("Cannot store key. This is a dummy read-only KeyStore")
}
