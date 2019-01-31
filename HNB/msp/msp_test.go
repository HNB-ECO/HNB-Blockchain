package msp

import (
	"fmt"
	"testing"
)

func TestSign(t *testing.T) {
	priKey, err := GeneratePriKey(1)
	if err != nil {
		fmt.Println("err1", err)
	}
	NewKeyPair()
	keyPair.PriKey = priKey
	pubKey, err := priKey.PublicKey()
	if err != nil {
		fmt.Println("err2", err)
	}
	keyPair.PubKey = pubKey
	keyPair.Scheme = 1
	data, err := Sign([]byte("123"))
	if err != nil {
		fmt.Println("err3", err)
	}
	fmt.Println(data)
	keyStr := PubKeyToString(keyPair.Scheme, keyPair.PubKey)
	keyO := StringToBccspKey(keyStr)
	ok, err := Verify(keyO, data, []byte("123"))
	fmt.Println(ok, err)
}
