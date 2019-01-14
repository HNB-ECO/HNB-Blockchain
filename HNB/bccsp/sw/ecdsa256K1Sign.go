package sw

import (
	"HNB/bccsp"
	"HNB/bccsp/secp256k1"
	"crypto/elliptic"
	"errors"
	//"HNB/bccsp/utils"
	//"crypto/ecdsa"
	"fmt"
)

type ecdsa256K1Signer struct{}

func (s *ecdsa256K1Signer) Sign(k bccsp.Key, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error) {

	if k == nil || digest == nil {
		return nil, errors.New("k or digest is nil")
	}

	blob, err := k.Bytes()
	if err != nil {
		return nil, err
	}

	privkey := make([]byte, 32)

	copy(privkey[32-len(blob):], blob)

	return secp256k1.Sign(digest, blob)
}

type ecdsaPublicKey256K1Verifier struct{}

func (v *ecdsaPublicKey256K1Verifier) Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {

	key, err := k.PublicKey()
	if err != nil {
		return false, nil
	}
	key1, ok := key.(*Ecdsa256K1PublicKey)
	if !ok {
		return false, errors.New("key type invalid")
	}
	pubkey := elliptic.Marshal(secp256k1.S256(), key1.PubKey.X, key1.PubKey.Y)

	return secp256k1.VerifySignature(pubkey, digest, signature), nil
}

type ecdsaPrivKey256K1Verifier struct{}

func (v *ecdsaPrivKey256K1Verifier) Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {

	key, err := k.PublicKey()
	if err != nil {
		return false, nil
	}
	key1, ok := key.(*Ecdsa256K1PublicKey)
	if !ok {
		return false, errors.New("key type invalid")
	}
	pubkey := elliptic.Marshal(secp256k1.S256(), key1.PubKey.X, key1.PubKey.Y)

	fmt.Printf("sign len:%v, hash len:%v\n", len(signature), len(digest))
	return secp256k1.VerifySignature(pubkey, digest, signature), nil
}
