package pkcs11

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"

	"HNB/bccsp"
)

type ecdsaSignature struct {
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

func marshalECDSASignature(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(ecdsaSignature{r, s})
}

func unmarshalECDSASignature(raw []byte) (*big.Int, *big.Int, error) {
	sig := new(ecdsaSignature)
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

func (csp *impl) signECDSA(k ecdsaPrivateKey, digest []byte, opts bccsp.SignerOpts) (signature []byte, err error) {
	r, s, err := csp.signP11ECDSA(k.ski, digest)
	if err != nil {
		return nil, err
	}

	halfOrder, ok := curveHalfOrders[k.pub.pub.Curve]
	if !ok {
		return nil, fmt.Errorf("Curve not recognized [%s]", k.pub.pub.Curve)
	}

	if s.Cmp(halfOrder) == 1 {
		s.Sub(k.pub.pub.Params().N, s)
	}

	return marshalECDSASignature(r, s)
}

func (csp *impl) verifyECDSA(k ecdsaPublicKey, signature, digest []byte, opts bccsp.SignerOpts) (valid bool, err error) {
	r, s, err := unmarshalECDSASignature(signature)
	if err != nil {
		return false, fmt.Errorf("Failed unmashalling signature [%s]", err)
	}

	halfOrder, ok := curveHalfOrders[k.pub.Curve]
	if !ok {
		return false, fmt.Errorf("Curve not recognized [%s]", k.pub.Curve)
	}

	if s.Cmp(halfOrder) == 1 {
		return false, fmt.Errorf("Invalid S. Must be smaller than half the order [%s][%s].", s, halfOrder)
	}

	if csp.softVerify {
		return ecdsa.Verify(k.pub, digest, r, s), nil
	} else {
		return csp.verifyP11ECDSA(k.ski, digest, r, s, k.pub.Curve.Params().BitSize/8)
	}
}
