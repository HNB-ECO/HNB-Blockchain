package rest

import (
	"HNB/ledger"
	"encoding/json"
	"fmt"
	"github.com/gocraft/web"
	"net/http"
	"strconv"
	"HNB/util"
)

func (*serverREST) BlockHeight(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	height, _ := ledger.GetBlockHeight()
	retMsg := FormatQueryResResult("0000", "", height)
	encoder.Encode(retMsg)
}

func (*serverREST) Block(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	blkNum := req.PathParams["blkNum"]
	height, _ := ledger.GetBlockHeight()
	blkInt, _ := strconv.ParseUint(blkNum, 10, 64)

	if blkInt+1 > height {
		msg := fmt.Sprintf("height %s < blkNum %s",
			strconv.FormatUint(height, 10), blkNum)
		retMsg := FormatQueryResResult("0001", msg, nil)
		encoder.Encode(retMsg)
		return
	}

	blkInfo, _ := ledger.GetBlock(blkInt)

	//m, _ := json.Marshal(blkInfo)
	retMsg := FormatQueryResResult("0000", "", blkInfo)
	encoder.Encode(retMsg)
}

func (*serverREST) TxHash(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	txHash := req.PathParams["txHash"]

	info, err := ledger.FindHashIndex(util.HexToByte(txHash))
	if err != nil {
		msg := fmt.Sprintf("txHash %s", err.Error())
		retMsg := FormatQueryResResult("0001", msg, nil)
		encoder.Encode(retMsg)
		return
	}

	if info == nil {
		msg := fmt.Sprintf("txHash:%s not exist", txHash)
		retMsg := FormatQueryResResult("0001", msg, nil)
		encoder.Encode(retMsg)
		return
	}

	tx, err := ledger.GetTransaction(info.BlockNum, info.Offset)
	if err != nil {
		msg := fmt.Sprintf("txHash %s", err.Error())
		retMsg := FormatQueryResResult("0001", msg, nil)
		encoder.Encode(retMsg)
		return
	}

	//m, _ := json.Marshal(tx)
	retMsg := FormatQueryResResult("0000", "", tx)
	encoder.Encode(retMsg)
}

