package sw

import (
	"errors"

	"crypto/sha256"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
)

type aesPrivateKey struct {
	privKey    []byte
	exportable bool
}

func (k *aesPrivateKey) Bytes() (raw []byte, err error) {
	if k.exportable {
		return k.privKey, nil
	}

	return nil, errors.New("Not supported.")
}

func (k *aesPrivateKey) SKI() (ski []byte) {
	hash := sha256.New()
	hash.Write([]byte{0x01})
	hash.Write(k.privKey)
	return hash.Sum(nil)
}

func (k *aesPrivateKey) Symmetric() bool {
	return true
}

func (k *aesPrivateKey) Private() bool {
	return true
}

func (k *aesPrivateKey) PublicKey() (bccsp.Key, error) {
	return nil, errors.New("Cannot call this method on a symmetric key.")
}
