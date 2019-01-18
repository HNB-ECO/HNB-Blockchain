package msp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp/secp256k1"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp/sw"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/cli/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/logging"
	"io/ioutil"
	"math/big"
	"os"
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
		key.D = new(big.Int)
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
	err = json.Unmarshal(data, &sKeyPair)
	if err != nil {
		return nil, err
	}
	return sKeyPair, nil
}

func Save(keypair *KeyPair, path string) error {
	priKeyBytes, err := GetPriKeyBytes(keypair.Scheme, keypair.PriKey)
	if err != nil {
		return err
	}

	pubKeyBytes, err := GetPubKeyBytes(keypair.Scheme, keypair.PubKey)
	if err != nil {
		return err
	}

	saveKP := &SaveKeyPair{
		Scheme: keypair.Scheme,
		PubKey: pubKeyBytes,
		PriKey: priKeyBytes,
	}
	data, err := json.Marshal(saveKP)
	if err != nil {
		return err
	}
	if common.FileExisted(path) {
		filename := path + "~"
		err := ioutil.WriteFile(filename, data, 0644)
		if err != nil {
			return err
		}
		return os.Rename(filename, path)
	} else {
		return ioutil.WriteFile(path, data, 0644)
	}
}
