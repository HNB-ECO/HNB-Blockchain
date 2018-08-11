package vrf

import (
	"fmt"
	"crypto/elliptic"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

var (
	curve   elliptic.Curve
	sk 		*ecdsa.PrivateKey
	pk 		*ecdsa.PublicKey
)

func initTest() {
	curve = elliptic.P256()
	var err error
	sk, err = ecdsa.GenerateKey(curve,rand.Reader)
	if err != nil {
		fmt.Printf("GenerateKey err %s\n", err.Error())
	}
	pk = &sk.PublicKey


}

func TestCalaAndVerfy(t *testing.T)  {
	initTest()
	preVrf := "1c9810aa9822e511d5804a9c4db9dd08497c31087b0daafa34d768a3253441fa20515e2f30f81741102af0ca3cefc4818fef16adb825fbaa8cad78647f3afb590e"
	value, err := hex.DecodeString(preVrf)
	if err != nil {
		fmt.Printf("DecodeString err %s\n", err.Error())
	}
	vrfValue, vrfProof, err := computeVrf(sk, 1, value)
	if err != nil {
		fmt.Printf("computeVrf err %s\n", err.Error())
	}
	fmt.Printf("vrfValue %s \nproof    %s\n", hex.EncodeToString(vrfValue), hex.EncodeToString(vrfProof))

	err = verify(pk, 1, value, vrfValue, vrfProof)
	if err != nil {
		fmt.Printf("verify err %s\n", err.Error())
	}

}
