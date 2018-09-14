package msp

import (
	"HNB/bccsp"
	"bytes"
	//"crypto/ecdsa"
	//"crypto/elliptic"
	//"encoding/json"
	//"errors"
	"fmt"
	//"math/big"
	"strconv"
	"HNB/bccsp/factory"
	"HNB/bccsp/sw"
	"crypto/ecdsa"
)

const (
	ECDSAP224 = 0
	ECDSAP256 = 1
	ECDSAP384 = 2
	ECDSAP521 = 3
	SM2       = 4
	RSA1024   = 5
	RSA2048   = 6
	ED25519   = 7
)





//byte转16进制字符串
func ByteToHex(data []byte) string {
	buffer := new(bytes.Buffer)
	for _, b := range data {

		s := strconv.FormatInt(int64(b&0xff), 16)
		if len(s) == 1 {
			buffer.WriteString("0")
		}
		buffer.WriteString(s)
	}

	return buffer.String()
}

//16进制字符串转[]byte
func HexToByte(hex string) []byte {
	length := len(hex) / 2
	slice := make([]byte, length)
	rs := []rune(hex)

	for i := 0; i < length; i++ {
		s := string(rs[i*2 : i*2+2])
		value, _ := strconv.ParseInt(s, 16, 10)
		slice[i] = byte(value & 0xFF)
	}
	return slice
}





func GeneratePriKey(algType int) (bccsp.Key, error) {

	bccspInstance := factory.GetDefault()

	switch algType {
	case ECDSAP256: //ECDSAP256-SHA256
		priKey, err := bccspInstance.KeyGen(&bccsp.ECDSAP256KeyGenOpts{Temporary:true})
		if err != nil {
			return nil, err
		}
		return priKey, nil

	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}

	return nil, nil
}

func ImportPriKey(algType int, priByte []byte) (bccsp.Key, error) {

	bccspInstance := factory.GetDefault()

	switch algType {

	case ECDSAP256: //ECDSAP256-SHA256
		priKey, err := bccspInstance.KeyImport(priByte, &bccsp.ECDSAPrivateKeyImportOpts{Temporary:true})
		if err != nil {
			return nil, err
		}
		return priKey, nil

	default: //
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}

	return nil, nil
}

func GethashFucName(algType int) string {
	hashFucName := ""

	switch algType {

	case ECDSAP256:
		hashFucName = bccsp.SHA256
	}

	if hashFucName == "" {
		fmt.Println("unknown type %d", algType)
		return ""
	}
	return hashFucName
}

func GetPriKey(algType int, key bccsp.Key ) (interface{}, error){
	switch algType  {

	case ECDSAP256:
		priKey := key.(*sw.EcdsaPrivateKey).PrivKey
		return priKey, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil

}

func GetPubKey(algType int, key bccsp.Key ) (interface{}, error){
	switch algType  {

	case ECDSAP256:
		pubKey := key.(*sw.EcdsaPublicKey).PubKey
		return pubKey, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil

}

func BuildPriKey(algType int, key interface{}) (bccsp.Key, error) {
	switch algType  {

	case ECDSAP256:
		priKey := key.(*ecdsa.PrivateKey)
		return &sw.EcdsaPrivateKey{PrivKey:priKey}, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil
}

func BuildPubKey(algType int, key interface{}) (bccsp.Key, error) {
	switch algType  {

	case ECDSAP256:
		pubKey := key.(*ecdsa.PublicKey)
		return &sw.EcdsaPublicKey{PubKey:pubKey}, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil
}




