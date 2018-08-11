package common

import (
	"encoding/json"
	"fmt"
	"testing"
	"encoding/hex"
)

func TestNewTransaction(t *testing.T) {
	tx := new(Transaction)
	tx.Txid = "hnb"
	tx.Txid = "asdasdasd"
	data, _ := json.Marshal(tx)
	fmt.Println(hex.EncodeToString(data))
}
