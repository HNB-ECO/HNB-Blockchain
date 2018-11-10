package rest

import (
	"HNB/common"
	"fmt"
)

type RetCode string
type RetMsg string

type RestInsertResult struct {
	RestBase
	Txid string `json:"txid,omitempty"`
}

type RestQueryResult struct {
	RestBase
	Data interface{} `json:"data,omitempty"`
}

type RestBase struct {
	Code    RetCode     `json:"code,omitempty"`
	Message interface{} `json:"msg,omitempty"`
}

func FormatInvokeResResult(code RetCode, message interface{}, txid common.Hash) *RestInsertResult {
	res := &RestInsertResult{}
	res.Code = code
	res.Message = message
	res.Txid = fmt.Sprintf("%x", txid)
	return res
}

func FormatQueryResResult(code RetCode, message interface{}, data interface{}) *RestQueryResult {

	res := &RestQueryResult{}
	res.Code = code
	res.Message = message
	res.Data = data
	return res
}
