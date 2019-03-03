package sw

import (
	"crypto/ecdsa"
	"fmt"

	"errors"
	"math/big"

	"crypto/hmac"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
)

type ecdsaPublicKeyKeyDeriver struct{}

func (kd *ecdsaPublicKeyKeyDeriver) KeyDeriv(k bccsp.Key, opts bccsp.KeyDerivOpts) (dk bccsp.Key, err error) {
	if opts == nil {
		return nil, errors.New("Invalid opts parameter. It must not be nil.")
	}

	ecdsaK := k.(*EcdsaPublicKey)

	switch opts.(type) {
	case *bccsp.ECDSAReRandKeyOpts:
		reRandOpts := opts.(*bccsp.ECDSAReRandKeyOpts)
		tempSK := &ecdsa.PublicKey{
			Curve: ecdsaK.PubKey.Curve,
			X:     new(big.Int),
			Y:     new(big.Int),
		}

		var k = new(big.Int).SetBytes(reRandOpts.ExpansionValue())
		var one = new(big.Int).SetInt64(1)
		n := new(big.Int).Sub(ecdsaK.PubKey.Params().N, one)
		k.Mod(k, n)
		k.Add(k, one)

		tempX, tempY := ecdsaK.PubKey.ScalarBaseMult(k.Bytes())
		tempSK.X, tempSK.Y = tempSK.Add(
			ecdsaK.PubKey.X, ecdsaK.PubKey.Y,
			tempX, tempY,
		)

		isOn := tempSK.Curve.IsOnCurve(tempSK.X, tempSK.Y)
		if !isOn {
			return nil, errors.New("Failed temporary public key IsOnCurve check.")
		}

		return &EcdsaPublicKey{tempSK}, nil
	default:
		return nil, fmt.Errorf("Unsupported 'KeyDerivOpts' provided [%v]", opts)
	}
}

type ecdsaPrivateKeyKeyDeriver struct{}

func (kd *ecdsaPrivateKeyKeyDeriver) KeyDeriv(k bccsp.Key, opts bccsp.KeyDerivOpts) (dk bccsp.Key, err error) {

	if opts == nil {
		return nil, errors.New("Invalid opts parameter. It must not be nil.")
	}

	ecdsaK := k.(*EcdsaPrivateKey)

	switch opts.(type) {

	case *bccsp.ECDSAReRandKeyOpts:
		reRandOpts := opts.(*bccsp.ECDSAReRandKeyOpts)
		tempSK := &ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: ecdsaK.PrivKey.Curve,
				X:     new(big.Int),
				Y:     new(big.Int),
			},
			D: new(big.Int),
		}

		var k = new(big.Int).SetBytes(reRandOpts.ExpansionValue())
		var one = new(big.Int).SetInt64(1)
		n := new(big.Int).Sub(ecdsaK.PrivKey.Params().N, one)
		k.Mod(k, n)
		k.Add(k, one)

		tempSK.D.Add(ecdsaK.PrivKey.D, k)
		tempSK.D.Mod(tempSK.D, ecdsaK.PrivKey.PublicKey.Params().N)

		tempX, tempY := ecdsaK.PrivKey.PublicKey.ScalarBaseMult(k.Bytes())
		tempSK.PublicKey.X, tempSK.PublicKey.Y =
			tempSK.PublicKey.Add(
				ecdsaK.PrivKey.PublicKey.X, ecdsaK.PrivKey.PublicKey.Y,
				tempX, tempY,
			)

		isOn := tempSK.Curve.IsOnCurve(tempSK.PublicKey.X, tempSK.PublicKey.Y)
		if !isOn {
			return nil, errors.New("Failed temporary public key IsOnCurve check.")
		}

		return &EcdsaPrivateKey{tempSK}, nil
	default:
		return nil, fmt.Errorf("Unsupported 'KeyDerivOpts' provided [%v]", opts)
	}
}

type aesPrivateKeyKeyDeriver struct {
	bccsp *impl
}

func (kd *aesPrivateKeyKeyDeriver) KeyDeriv(k bccsp.Key, opts bccsp.KeyDerivOpts) (dk bccsp.Key, err error) {
	if opts == nil {
		return nil, errors.New("Invalid opts parameter. It must not be nil.")
	}

	aesK := k.(*aesPrivateKey)

	switch opts.(type) {
	case *bccsp.HMACTruncated256AESDeriveKeyOpts:
		hmacOpts := opts.(*bccsp.HMACTruncated256AESDeriveKeyOpts)

		mac := hmac.New(kd.bccsp.conf.hashFunction, aesK.privKey)
		mac.Write(hmacOpts.Argument())
		return &aesPrivateKey{mac.Sum(nil)[:kd.bccsp.conf.aesBitLength], false}, nil

	case *bccsp.HMACDeriveKeyOpts:
		hmacOpts := opts.(*bccsp.HMACDeriveKeyOpts)

		mac := hmac.New(kd.bccsp.conf.hashFunction, aesK.privKey)
		mac.Write(hmacOpts.Argument())
		return &aesPrivateKey{mac.Sum(nil), true}, nil
	default:
		return nil, fmt.Errorf("Unsupported 'KeyDerivOpts' provided [%v]", opts)
	}
}
