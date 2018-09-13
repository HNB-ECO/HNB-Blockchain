package msp

import (
	"HNB/bccsp"
	"HNB/bccsp/sw"
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"fmt"

	"github.com/juju/errors"
	"HNB/bccsp/factory"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)



func GetBccspKeyFromPubKey(pubkey string) bccsp.Key{

	key, err := PubKeyDecode(keyPair.Scheme, pubkey)
	if err != nil {
		MSPLog.Errorf(LOGTABLE_MSP, "PubKeyDecode %s", err.Error())
		return nil
	}
	return key
}

func GetPeerPubStr() string{
	return PubKeyToString(keyPair.Scheme, keyPair.PubKey)
}

func GetPubStrFromO(Scheme int, pubKey bccsp.Key) string{
	return PubKeyToString(Scheme, pubKey)
}

func GetPeerID() []byte {
	ID, err := hex.DecodeString(GetPeerPubStr())
	if err != nil {
		MSPLog.Errorf(LOGTABLE_MSP, "DecodeString %s", err.Error())
		return nil
	}
	return ID
}
func GetPubStrFromPeerID(ID []byte) string {
	return hex.EncodeToString(ID)
}

func GetPubBytesFromStr(pubKeyStr string) []byte{
	id,_ := hex.DecodeString(pubKeyStr)
	return id
}


func Hash256(msg []byte) ([]byte, error){
	hash256 := sha256.New()
	hash256.Write(msg)
	return hash256.Sum(nil), nil
}

func Sign(msg []byte) (signature []byte, err error) {
	bccspInstance := factory.GetDefault()

	if msg == nil  {
		return nil, errors.New("msg  is nil")
	}

	digest, err := Hash(msg)
	if err != nil {
		return nil, err
	}

	switch keyPair.Scheme {
	case ECDSAP256:
		return bccspInstance.Sign(keyPair.PriKey, digest, nil)

	default:
		return nil, fmt.Errorf("private key %v not supported", keyPair.Scheme)
	}
}


func Verify(pubKey bccsp.Key, signature, msg []byte) (valid bool, err error) {
	bccspInstance := factory.GetDefault()

	if pubKey == nil || signature == nil || msg == nil {
		return false, errors.New("signature or private key or msg or hashFuncName is nil")
	}

	digest, err := Hash(msg)
	if err != nil {
		return false, err
	}

	switch keyPair.Scheme {
	case ECDSAP256:
		return bccspInstance.Verify(pubKey, signature, digest, nil)

	default:
		return false, fmt.Errorf("private key %v not supported", pubKey)
	}
}


func PubKeyToString(keyType int, pubKey interface{}) string {

	switch keyType {

	case ECDSAP256:
		keyInterf := pubKey.(*sw.EcdsaPublicKey).PubKey
		return PubKeyToString2(keyType, keyInterf)

	}
	return ""
}

func PubKeyToString2(keyType int, CommPub interface{}) string {
	switch keyType {

	case ECDSAP256:
		return GenKey(256, CommPub)

	default:

	}

	return ""

}

func GenKey(algLength int, CommPub interface{}) string {
	pubKey := CommPub.(*ecdsa.PublicKey)

	xPubKey := pubKey.X.Text(16)
	yPubKey := pubKey.Y.Text(16)

	xLenPK := len(xPubKey)
	yLenPK := len(yPubKey)

	xZero := (algLength/8)*2 - xLenPK
	yZero := (algLength/8)*2 - yLenPK

	var zero string
	for i := 0; i < xZero; i++ {
		zero += "0"
	}

	xPubKey = zero + xPubKey

	zero = ""
	for i := 0; i < yZero; i++ {
		zero += "0"
	}
	yPubKey = zero + yPubKey

	return xPubKey + yPubKey
}


func Hash(msg []byte) (digest []byte, err error) {
	bccspInstance := factory.GetDefault()

	if msg == nil {
		return nil, errors.New("msg is nil")
	}

	switch keyPair.Scheme {

	case ECDSAP256:
		return bccspInstance.Hash(msg, &bccsp.SHA256Opts{})
	default:
		return nil, fmt.Errorf("%s not supported", keyPair.Scheme)
	}
}


