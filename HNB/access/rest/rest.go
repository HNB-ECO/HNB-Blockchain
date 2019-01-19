package rest

import (
	"HNB/config"
	"HNB/logging"
	"github.com/gocraft/web"
	"net/http"
	"strconv"
	"encoding/json"
	"io/ioutil"
	"fmt"
)

var RestLog logging.LogModule

const (
	LOGTABLE_REST string = "rest"
	RPCVERSION           = "1.0"
)

var router *web.Router

type serverREST struct {
}

func (s *serverREST) SetResponseType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-ContractName", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "accept, content-type")
	next(rw, req)
}

// 构建router & 向router中注册url
func buildOpenchainRESTRouter() *web.Router {
	s := serverREST{}
	router = web.New(s)
	router.Middleware((*serverREST).SetResponseType)
	router.Post("/", (*serverREST).Process)
	//router.Get("/getaddr", (*serverREST).GetAddr)
	//router.Get("/querybalance/:chainID/:addr", (*serverREST).QueryBalanceMsg)
	//router.Post("/sendtxmsg", (*serverREST).SendTxMsg)
	//router.Get("/blockheight", (*serverREST).HighestBlockNum)
	//router.Get("/block/:blkNum", (*serverREST).Block)
	//router.Get("/querytx/:txHash", (*serverREST).TxHash)
	//router.Get("/querytxpool", (*serverREST).GetTxPoolQueue)
	return router
}
func (*serverREST) Process(rw web.ResponseWriter, req *web.Request) {
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	reqBody, _ := ioutil.ReadAll(req.Body)
	jm := JsonrpcMessage{}
	err := json.Unmarshal(reqBody, &jm)
	if err != nil {
		msg := fmt.Sprintf("reqBody err:%v", err.Error())
		retMsg := ErrorResponse(RPCVERSION, 0, 0, msg)
		encoder.Encode(retMsg)
		return
	}

	if jm.Version != RPCVERSION {
		retMsg := ErrorResponse(RPCVERSION, jm.ID, 1, "rpcversion invalid")
		encoder.Encode(retMsg)
		return
	}

	function,ok := funcList[jm.Method]
	if !ok{
		retMsg := ErrorResponse(RPCVERSION, jm.ID, 2, "method invalid")
		encoder.Encode(retMsg)
		return
	}

	data, err := function(jm.Params)
	if err != nil{
		retMsg := ErrorResponse(RPCVERSION, jm.ID, 3, err.Error())
		encoder.Encode(retMsg)
		return
	}

	retMsg := SuccessResponse(RPCVERSION, jm.ID, data)
	encoder.Encode(retMsg)
}


var funcList = map[string] func(params json.RawMessage) (interface{}, error){
	"blockNumber":        HighestBlockNum,
	"getBlockByNumber":   Block,
	"getBlockByHash":     TxHash,
	"getAddr":            GetAddr,
	"getTxPool":          GetTxPoolQueue,
	"getBalance":         QueryBalanceMsg,
	"sendRawTransaction": SendTxMsg,
}

func StartRESTServer() {
	router := buildOpenchainRESTRouter()
	port := strconv.FormatUint(uint64(config.Config.RestPort), 10)

	RestLog = logging.GetLogIns()
	RestLog.Info(LOGTABLE_REST, "rest start port: "+port)

	//函数注册


	go func() {
		http.ListenAndServe(":"+port, router)
		err := http.ListenAndServe(":"+port, router)
		if err != nil {
			panic(err)
		}
	}()
}
