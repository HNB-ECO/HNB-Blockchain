package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
)

func TestNewTransaction(t *testing.T) {
	tx := new(Transaction)
	tx.Txid = "hgs"
	data, _ := json.Marshal(tx)
	fmt.Println(hex.EncodeToString(data))
}
