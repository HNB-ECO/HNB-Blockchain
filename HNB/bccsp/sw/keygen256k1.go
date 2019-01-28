package sw

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"

	"HNB/bccsp"
	"HNB/bccsp/secp256k1"
)

type ecdsa256K1KeyGenerator struct {
	curve elliptic.Curve
}

func (kg *ecdsa256K1KeyGenerator) KeyGen(opts bccsp.KeyGenOpts) (k bccsp.Key, err error) {
	key, err := ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("Failed generating ECDSA key for [%v]: [%s]", kg.curve, err)
	}

	return &Ecdsa256K1PrivateKey{key}, nil
}
