package sw

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"

	"crypto/sha256"

	"errors"

	"crypto/elliptic"

	"HNB/bccsp"
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
	return &EcdsaPublicKey{&k.PrivKey.PublicKey}, nil
}

type EcdsaPublicKey struct {
	PubKey *ecdsa.PublicKey
}

func (k *EcdsaPublicKey) Bytes() (raw []byte, err error) {
	raw, err = x509.MarshalPKIXPublicKey(k.PubKey)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling key [%s]", err)
	}
	return
}

func (k *EcdsaPublicKey) SKI() (ski []byte) {
	if k.PubKey == nil {
		return nil
	}

	raw := elliptic.Marshal(k.PubKey.Curve, k.PubKey.X, k.PubKey.Y)

	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
}

func (k *EcdsaPublicKey) Symmetric() bool {
	return false
}

func (k *EcdsaPublicKey) Private() bool {
	return false
}

func (k *EcdsaPublicKey) PublicKey() (bccsp.Key, error) {
	return k, nil
}
