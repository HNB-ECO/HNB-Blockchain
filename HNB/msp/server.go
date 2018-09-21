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

func PubKeyDecode(algType int, codeStr string) (pubKey bccsp.Key, err error) {

        var paramS string
        switch algType {

        case ECDSAP256:
                if paramS == "" {
                        paramS = "{\"P\":115792089210356248762697446949407573530086143415290314195533631308867097853951,\"N\":115792089210356248762697446949407573529996955224135760342422259061068512044369,\"B\":41058363725152142129326129780047268409114441015993725554835256314039467401291,\"Gx\":48439561293906451759052585252797914202762949526041747995844080717082404635286,\"Gy\":36134250956749795798585127919587881956611106672985015071877198253568414405109,\"BitSize\":256,\"Name\":\"P-256\"}"
                }
                param := &elliptic.CurveParams{}
                json.Unmarshal([]byte(paramS), param)
                algLength := 256
                fmt.Printf("bitsize:%d, key:%s\n", algLength, codeStr)

                pk := &ecdsa.PublicKey{X: &big.Int{}, Y: &big.Int{}}

                pk.Curve = param

                /*algLength/8 transfer byte, X,byte to string double char*/
                pk.X.SetString(codeStr[0:((algLength+7)/8)*2], 16)

                /*algLength/8 transfer byte, Y,byte to string double char*/
                pk.Y.SetString(codeStr[((algLength+7)/8)*2:((algLength+7)/8)*2*2], 16)

                return &sw.EcdsaPublicKey{PubKey: pk}, nil

        }
        return nil, errors.New("algType not exist")
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


