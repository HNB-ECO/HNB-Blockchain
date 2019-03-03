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

type Ecdsa256K1PrivateKey struct {
	PrivKey *ecdsa.PrivateKey
}

func (k *Ecdsa256K1PrivateKey) Bytes() (raw []byte, err error) {
	if k.PrivKey == nil || k.PrivKey.D == nil {
		return nil, errors.New("PrivKey is nil")
	}

	return k.PrivKey.D.Bytes(), nil
}

func (k *Ecdsa256K1PrivateKey) SKI() (ski []byte) {
	if k.PrivKey == nil {
		return nil
	}

	raw := elliptic.Marshal(k.PrivKey.Curve, k.PrivKey.PublicKey.X, k.PrivKey.PublicKey.Y)

	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
}

func (k *Ecdsa256K1PrivateKey) Symmetric() bool {
	return false
}

func (k *Ecdsa256K1PrivateKey) Private() bool {
	return true
}

func (k *Ecdsa256K1PrivateKey) PublicKey() (bccsp.Key, error) {
	return &Ecdsa256K1PublicKey{&k.PrivKey.PublicKey}, nil
}

type Ecdsa256K1PublicKey struct {
	PubKey *ecdsa.PublicKey
}

func (k *Ecdsa256K1PublicKey) Bytes() (raw []byte, err error) {
	raw, err = x509.MarshalPKIXPublicKey(k.PubKey)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling key [%s]", err)
	}
	return
}

func (k *Ecdsa256K1PublicKey) SKI() (ski []byte) {
	if k.PubKey == nil {
		return nil
	}

	raw := elliptic.Marshal(k.PubKey.Curve, k.PubKey.X, k.PubKey.Y)

	hash := sha256.New()
	hash.Write(raw)
	return hash.Sum(nil)
}

func (k *Ecdsa256K1PublicKey) Symmetric() bool {
	return false
}

func (k *Ecdsa256K1PublicKey) Private() bool {
	return false
}

func (k *Ecdsa256K1PublicKey) PublicKey() (bccsp.Key, error) {
	return k, nil
}
