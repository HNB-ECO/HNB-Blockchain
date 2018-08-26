package common

import (
	"testing"

	"time"

	"fmt"
	"uchains/api/util"

	"golang.org/x/crypto/ripemd160"
)

type vote struct {
	Height    uint64    `json:"height"`
	Timestamp time.Time `json:"timestamp"`
}

func Test_Hash(t *testing.T) {
	var t1 = time.Now()
	for i := 0; i < 10; i++ {

		vote := vote{
			Height:    6,
			Timestamp: t1,
		}
		hasher := hasher{item: vote}

		fmt.Println(util.ByteToHex(hasher.Hash2()))
	}

}

func (h hasher) Hash2() []byte {
	hasher := ripemd160.New()
	if h.item != nil && !IsTypedNil(h.item) && !IsEmpty(h.item) {
		bz, err := Hash256Byte2(h.item)
		if err != nil {
			panic(err)
		}
		_, err = hasher.Write(bz)
		if err != nil {
			panic(err)
		}
	}

	return hasher.Sum(nil)
}
