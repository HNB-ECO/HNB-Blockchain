
package pkcs11

import (
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
	"fmt"

	"HNB/bccsp"
)

type ecdsaPrivateKey struct {
	ski []byte
	pub ecdsaPublicKey
}

func (k *ecdsaPrivateKey) Bytes() (raw []byte, err error) {
	return nil, errors.New("Not supported.")
}

func (k *ecdsaPrivateKey) SKI() (ski []byte) {
	return k.ski
}

func (k *ecdsaPrivateKey) Symmetric() bool {
	return false
}

func (k *ecdsaPrivateKey) Private() bool {
	return true
}

func (k *ecdsaPrivateKey) PublicKey() (bccsp.Key, error) {
	return &k.pub, nil
}

type ecdsaPublicKey struct {
	ski []byte
	pub *ecdsa.PublicKey
}

func (k *ecdsaPublicKey) Bytes() (raw []byte, err error) {
	raw, err = x509.MarshalPKIXPublicKey(k.pub)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling key [%s]", err)
	}
	return
}

func (k *ecdsaPublicKey) SKI() (ski []byte) {
	return k.ski
}

func (k *ecdsaPublicKey) Symmetric() bool {
	return false
}

func (k *ecdsaPublicKey) Private() bool {
	return false
}

func (k *ecdsaPublicKey) PublicKey() (bccsp.Key, error) {
	return k, nil
}
