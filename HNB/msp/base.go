package msp

import (
	"HNB/bccsp"
	"HNB/bccsp/secp256k1"
	"HNB/bccsp/sw"
	"HNB/logging"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type KeyPair struct {
	Scheme int
	PubKey bccsp.Key
	PriKey bccsp.Key
}
type SaveKeyPair struct {
	Scheme int
	PubKey []byte
	PriKey []byte
}

var MSPLog logging.LogModule
var keyPair *KeyPair

const (
	LOGTABLE_MSP string = "msp"
)

func GetLocalPrivKey() bccsp.Key {
	return keyPair.PriKey
}
func NewKeyPair() *KeyPair {
	MSPLog = logging.GetLogIns()
	ky := &KeyPair{}
	keyPair = ky
	return ky
}
func (kp *KeyPair) Init(path string) error {
	sKeyPair, err := Load(path)
	if err != nil {
		return err
	}

	algType := sKeyPair.Scheme
	switch algType {
	case ECDSAP256:
		key := new(ecdsa.PrivateKey)
		key.Curve = secp256k1.S256()
		key.D.SetBytes(sKeyPair.PriKey)
		key.PublicKey.X, key.PublicKey.Y = elliptic.Unmarshal(secp256k1.S256(), sKeyPair.PubKey)
		kp.Scheme = sKeyPair.Scheme
		kp.PubKey = &sw.Ecdsa256K1PublicKey{&key.PublicKey}
		kp.PriKey = &sw.Ecdsa256K1PrivateKey{key}
		return nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return fmt.Errorf("algType not support")
	}
	return nil
}

func Load(path string) (*SaveKeyPair, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sKeyPair := new(SaveKeyPair)
	err = json.Unmarshal(data, sKeyPair)
	if err != nil {
		return nil, err
	}
	return sKeyPair, nil
}
