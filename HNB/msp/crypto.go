package msp

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp/factory"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp/secp256k1"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/bccsp/sw"
	"bytes"
	"crypto/elliptic"
	"fmt"
	"strconv"
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
		priKey, err := bccspInstance.KeyGen(&bccsp.ECDSAP256K1KeyGenOpts{Temporary: true})
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

func GetPriKey(algType int, key bccsp.Key) (interface{}, error) {
	switch algType {

	case ECDSAP256:
		priKey := key.(*sw.Ecdsa256K1PrivateKey).PrivKey
		return priKey, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil
}

func GetPriKeyBytes(algType int, key bccsp.Key) ([]byte, error) {
	switch algType {

	case ECDSAP256:
		priKey := key.(*sw.Ecdsa256K1PrivateKey).PrivKey.D.Bytes()
		return priKey, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil
}

func GetPubKey(algType int, key bccsp.Key) (interface{}, error) {
	switch algType {

	case ECDSAP256:
		pubKey := key.(*sw.Ecdsa256K1PublicKey).PubKey
		return pubKey, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil

}

func GetPubKeyBytes(algType int, key bccsp.Key) ([]byte, error) {
	switch algType {
	case ECDSAP256:
		pk := key.(*sw.Ecdsa256K1PublicKey)
		pubkeyBytes := elliptic.Marshal(secp256k1.S256(), pk.PubKey.X, pk.PubKey.Y)
		return pubkeyBytes, nil
	default:
		fmt.Printf("algType not support : %v\n", algType)
		return nil, fmt.Errorf("algType not support")
	}
	return nil, nil

}
