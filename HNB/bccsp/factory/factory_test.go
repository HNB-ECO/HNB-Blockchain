package factory

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
	"bytes"
	"fmt"
	"testing"
)

func TestGetDefault(t *testing.T) {
	bccsp := GetDefault()
	if bccsp == nil {
		fmt.Println("lala")
	} else {
		fmt.Println("jiji")
	}

}
func TestSecp256K1(t *testing.T) {
	bi := GetDefault()
	key, err := bi.KeyGen(&bccsp.ECDSAP256K1KeyGenOpts{Temporary: true})
	if err != nil {
		t.Error(err)
	}

	//私钥对象
	keyBytes, err := key.Bytes()
	if err != nil {
		t.Error(err)
	}

	key1, err := bi.KeyImport(keyBytes, &bccsp.ECDSAPrivateKey256K1ImportOpts{true})
	if err != nil {
		t.Error(err)
	}

	if bytes.Compare(key.SKI(), key1.SKI()) != 0 {
		t.Error("key import ski not same")
	}

	msg := []byte("hello")

	hash, err := bi.Hash(msg, &bccsp.SHA256Opts{})
	if err != nil {
		t.Error(err)
	}

	//k1, _ := key.Bytes()
	//k2, _ := key1.Bytes()
	//t.Logf("key %x", k1)
	//t.Logf("key1 %x", k2)

	v, err := bi.Sign(key, hash, nil)
	if err != nil {
		t.Error(err)
	}
	sv := v[:len(v)-1]

	fmt.Printf("sign len:%v\n", len(sv))
	r, err := bi.Verify(key, sv, hash, nil)
	if err != nil {
		t.Error(err)
	}
	if r == false {
		t.Error("sign fail")
	}

	t.Log("success")
}
