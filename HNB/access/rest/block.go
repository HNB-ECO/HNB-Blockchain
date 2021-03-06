package rest

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
	"bytes"
	"encoding/json"
	"fmt"
	"errors"
	"strconv"
)

func HighestBlockNum(params json.RawMessage) (interface{}, error) {
	//fmt.Printf("params:%v\n", string(params))

	//dec := json.NewDecoder(bytes.NewReader(params))

	//if tok, _ := dec.Token(); tok != json.Delim('[') {
	//	return nil, errors.New("no [")
	//}

	height, _ := ledger.GetBlockHeight()
	return height - 1, nil
}

func Block(params json.RawMessage) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(params))

	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, errors.New("no [")
	}

	if !dec.More() {
		return nil, errors.New("data not complete")
	}
	var blkNum uint64
	err := dec.Decode(&blkNum)
	if err != nil {
		return nil, err
	}
	height, _ := ledger.GetBlockHeight()

	if blkNum+1 > height {
		msg := fmt.Sprintf("height %s , blkNum %d",
			strconv.FormatUint(height, 10), blkNum)
		return nil, errors.New(msg)
	}

	blkInfo, err := ledger.GetBlock(blkNum)
	return blkInfo, nil
}




func TxHashResult(params json.RawMessage) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(params))

	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, errors.New("no [")
	}

	if !dec.More() {
		return nil, errors.New("data not complete")
	}
	var txHash string
	err := dec.Decode(&txHash)
	if err != nil {
		return nil, err
	}

	reason, err := ledger.GetWrongIndex(util.HexToByte(txHash))
	if err != nil {
		return nil, err
	}

	if reason != ""{
		return reason, nil
	}

	info, err := ledger.FindHashIndex(util.HexToByte(txHash))
	if err != nil {
		return nil, err
	}

	if info != nil {
		return "txid success", nil
	}


	return "txid not exist", nil
}
func TxHash(params json.RawMessage) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(params))

	if tok, _ := dec.Token(); tok != json.Delim('[') {
		return nil, errors.New("no [")
	}

	if !dec.More() {
		return nil, errors.New("data not complete")
	}
	var txHash string
	err := dec.Decode(&txHash)
	if err != nil {
		return nil, err
	}
	info, err := ledger.FindHashIndex(util.HexToByte(txHash))
	if err != nil {
		return nil, err
	}

	if info == nil {
		return nil, nil
	}

	tx, err := ledger.GetTransaction(info.BlockNum, info.Offset)
	if err != nil {
		return nil, err
	}


	return tx, nil
}
