
package sw

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"

	"crypto/sha256"

	"errors"

	"crypto/elliptic"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
)

type EcdsaPrivateKey struct {
	PrivKey *ecdsa.PrivateKey
}

func (k *EcdsaPrivateKey) Bytes() (raw []byte, err error) {
	return nil, errors.New("Not supported.")
}

func (k *EcdsaPrivateKey) SKI() (ski []byte) {
	if k.PrivKey == nil {
		return nil
	}

	raw := elliptic.Marshal(k.PrivKey.Curve, k.PrivKey.PublicKey.X, k.PrivKey.PublicKey.Y)

	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
}

func (k *EcdsaPrivateKey) Symmetric() bool {
	return false
}

func (k *EcdsaPrivateKey) Private() bool {
	return true
}

func (k *EcdsaPrivateKey) PublicKey() (bccsp.Key, error) {
	return &ecdsaPublicKey{&k.PrivKey.PublicKey}, nil
}

type ecdsaPublicKey struct {
	pubKey *ecdsa.PublicKey
}

func (k *ecdsaPublicKey) Bytes() (raw []byte, err error) {
	raw, err = x509.MarshalPKIXPublicKey(k.pubKey)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling key [%s]", err)
	}
	return
}

func (k *ecdsaPublicKey) SKI() (ski []byte) {
	if k.pubKey == nil {
		return nil
	}

	raw := elliptic.Marshal(k.pubKey.Curve, k.pubKey.X, k.pubKey.Y)

	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
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
