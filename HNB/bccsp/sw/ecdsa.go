package sw

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"

	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
)

type ECDSASignature struct {
	R, S *big.Int
}

var (
	curveHalfOrders map[elliptic.Curve]*big.Int = map[elliptic.Curve]*big.Int{
		elliptic.P224(): new(big.Int).Rsh(elliptic.P224().Params().N, 1),
		elliptic.P256(): new(big.Int).Rsh(elliptic.P256().Params().N, 1),
		elliptic.P384(): new(big.Int).Rsh(elliptic.P384().Params().N, 1),
		elliptic.P521(): new(big.Int).Rsh(elliptic.P521().Params().N, 1),
	}
)

func MarshalECDSASignature(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(ECDSASignature{r, s})
}

func UnmarshalECDSASignature(raw []byte) (*big.Int, *big.Int, error) {

	sig := new(ECDSASignature)
	_, err := asn1.Unmarshal(raw, sig)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed unmashalling signature [%s]", err)
	}

	if sig.R == nil {
		return nil, nil, errors.New("Invalid signature. R must be different from nil.")
	}
	if sig.S == nil {
		return nil, nil, errors.New("Invalid signature. S must be different from nil.")
	}

	if sig.R.Sign() != 1 {
		return nil, nil, errors.New("Invalid signature. R must be larger than zero")
	}
	if sig.S.Sign() != 1 {
		return nil, nil, errors.New("Invalid signature. S must be larger than zero")
	}

	return sig.R, sig.S, nil
}

func signECDSA(k *ecdsa.PrivateKey, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error) {
	r, s, err := ecdsa.Sign(rand.Reader, k, digest)
	if err != nil {
		return nil, err
	}

	s, _, err = ToLowS(&k.PublicKey, s)
	if err != nil {
		return nil, err
	}

	return MarshalECDSASignature(r, s)
}

func verifyECDSA(k *ecdsa.PublicKey, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {
	r, s, err := UnmarshalECDSASignature(signature)
	if err != nil {
		return false, fmt.Errorf("Failed unmashalling signature [%s]", err)
	}

	lowS, err := IsLowS(k, s)
	if err != nil {
		return false, err
	}

	if !lowS {
		return false, fmt.Errorf("Invalid S. Must be smaller than half the order [%s][%s].", s, curveHalfOrders[k.Curve])
	}

	return ecdsa.Verify(k, digest, r, s), nil
}

func SignatureToLowS(k *ecdsa.PublicKey, signature []byte) ([]byte, error) {
	r, s, err := UnmarshalECDSASignature(signature)
	if err != nil {
		return nil, err
	}

	s, modified, err := ToLowS(k, s)
	if err != nil {
		return nil, err
	}

	if modified {
		return MarshalECDSASignature(r, s)
	}

	return signature, nil
}

func IsLowS(k *ecdsa.PublicKey, s *big.Int) (bool, error) {
	var curve elliptic.Curve
	switch k.Curve.Params().BitSize {
	case 224:
		curve = elliptic.P224()
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	}
	halfOrder, ok := curveHalfOrders[curve]
	if !ok {
		return false, fmt.Errorf("Curve not recognized [%s]", k.Curve)
	}

	return s.Cmp(halfOrder) != 1, nil

}

func ToLowS(k *ecdsa.PublicKey, s *big.Int) (*big.Int, bool, error) {
	lowS, err := IsLowS(k, s)
	if err != nil {
		return nil, false, err
	}

	if !lowS {
		s.Sub(k.Params().N, s)

		return s, true, nil
	}

	return s, false, nil
}

type ecdsaSigner struct{}

func (s *ecdsaSigner) Sign(k bccsp.Key, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error) {
	return signECDSA(k.(*EcdsaPrivateKey).PrivKey, digest, opts)
}

type ecdsaPrivateKeyVerifier struct{}

func (v *ecdsaPrivateKeyVerifier) Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {
	return verifyECDSA(&(k.(*EcdsaPrivateKey).PrivKey.PublicKey), signature, digest, opts)
}

type ecdsaPublicKeyKeyVerifier struct{}

func (v *ecdsaPublicKeyKeyVerifier) Verify(k bccsp.Key, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {
	return verifyECDSA(k.(*EcdsaPublicKey).PubKey, signature, digest, opts)
}
