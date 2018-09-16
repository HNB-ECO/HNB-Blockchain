package msp

import (
	"HNB/bccsp"
	"io/ioutil"
	"encoding/json"
	//"HNB/bccsp/factory"
	"HNB/bccsp/utils"
	//"HNB/bccsp/sw"
	"HNB/logging"
)

type KeyPair struct {
	Scheme		int
	PubKey		bccsp.Key
	PriKey		bccsp.Key
}
type SaveKeyPair struct {
	Scheme		int
	PubKey		[]byte
	PriKey		[]byte
}
var MSPLog logging.LogModule
var keyPair *KeyPair
const(
	LOGTABLE_MSP string = "msp"

)

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

	sPriKey, err := utils.PEMtoPrivateKey(sKeyPair.PriKey, nil)
	if err != nil {
		return err
	}
	sk, err := BuildPriKey(sKeyPair.Scheme, sPriKey)
	if err != nil {
		return err
	}
	//sk, err := ImportPriKey(sKeyPair.Scheme, sPriKey)
	//if err != nil {
	//	return err
	//}

	//ski, err := ioutil.ReadFile("ID")
	//if err != nil {
	//	return err
	//}
	//keyStore, err := sw.NewFileBasedKeyStore(nil, path, false)
	//if err != nil {
	//	return err
	//}
	//sk, err := keyStore.GetKey(ski)
	pubKey, err := sk.PublicKey()
	if err != nil {
		return err
	}
	kp.Scheme = sKeyPair.Scheme
	kp.PubKey = pubKey
	kp.PriKey = sk
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
