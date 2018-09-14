package common

import (
	"bccsp"
	"crypto/sha256"
	"encoding/json"
)

func Hash256Byte(v interface{}) []byte {
	data, _ := json.Marshal(v)
	hashx := sha256.New()
	hashx.Write(data)
	return hashx.Sum(nil)
}
func Hash256Byte2(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	hashx := sha256.New()
	hashx.Write(data)
	return hashx.Sum(nil), nil
}

type hasher struct {
	item interface{}
}

func (h hasher) Hash() []byte {
	if h.item != nil && !IsTypedNil(h.item) && !IsEmpty(h.item) {

		bytes, err := json.Marshal(h.item)
		if err != nil {
			return nil
		}

		retBytes, err := bccsp.Hash(bytes, bccsp.SHA256)
		if err != nil {
			return nil
		}

		return retBytes
	}

	return nil
}

func AminoHash(item interface{}) []byte {
	h := hasher{item}
	return h.Hash()
}

func AminoHasher(item interface{}) merkle.Hasher {
	return hasher{item}
}
