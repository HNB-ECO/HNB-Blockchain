package msp

import (
	"HNB/bccsp"
	"HNB/bccsp/sw"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"

	"HNB/bccsp/factory"
	"HNB/bccsp/secp256k1"
	"crypto/sha256"
	"github.com/juju/errors"
)

/*
	公钥转字符串
*/
func PubKeyToString(keyType int, pubKey interface{}) string {

	switch keyType {

	case ECDSAP256:
		key := pubKey.(*sw.Ecdsa256K1PublicKey).PubKey
		pkBytes := elliptic.Marshal(secp256k1.S256(), key.X, key.Y)
		return ByteToHex(pkBytes)
	}
	return ""
}

func StringToBccspKey(pubkey string) bccsp.Key {

	keyType := keyPair.Scheme
	switch keyType {
	case ECDSAP256:
		pkEcc := new(ecdsa.PublicKey)
		pkEcc.X, pkEcc.Y = elliptic.Unmarshal(secp256k1.S256(), HexToByte(pubkey))
		pk := sw.Ecdsa256K1PublicKey{pkEcc}
		return &pk
	}

	return nil
}

func GetPeerPubStr() string {
	return PubKeyToString(keyPair.Scheme, keyPair.PubKey)
}

func BccspKeyToString(Scheme int, pubKey bccsp.Key) string {
	return PubKeyToString(Scheme, pubKey)
}

func GetPeerID() []byte {
	ID := HexToByte(GetPeerPubStr())
	return ID
}

func PeerIDToString(ID []byte) string {
	return ByteToHex(ID)
}

func StringToByteKey(pubKeyStr string) []byte {
	id := HexToByte(pubKeyStr)
	return id
}

func CalcAddrFromPubBytes(PubKeyBytes []byte) ([]byte, error) {

	return DoubleHash256(PubKeyBytes)
}

func Sign(msg []byte) (signature []byte, err error) {
	bccspInstance := factory.GetDefault()

	if msg == nil {
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
		signV := signature[:len(signature)-1]
		return bccspInstance.Verify(pubKey, signV, digest, nil)

	default:
		return false, fmt.Errorf("private key %v not supported", pubKey)
	}
}

func Hash256(msg []byte) ([]byte, error) {
	hash256 := sha256.New()
	hash256.Write(msg)
	return hash256.Sum(nil), nil
}

func DoubleHash256(msg []byte) ([]byte, error) {
	hash, _ := Hash256(msg)
	return Hash256(hash)
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
		return nil, fmt.Errorf("%d not supported", keyPair.Scheme)
	}
}

func GetAlgType() int {
	return keyPair.Scheme
}

//func rlpHash(x interface{}) (h []byte) {
//	hw := sha3.NewKeccak256()
//	rlp.Encode(hw, x)
//	hw.Sum(h[:0])
//	return h
//}
