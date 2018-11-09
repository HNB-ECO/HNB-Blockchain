package rest

import (
	"HNB/config"
	"HNB/logging"
	"github.com/gocraft/web"
	"net/http"
	"strconv"
)

var RestLog logging.LogModule

const (
	LOGTABLE_REST string = "rest"
)

var router *web.Router

type serverREST struct {
}

func (s *serverREST) SetResponseType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "accept, content-type")
	next(rw, req)
}

// 构建router & 向router中注册url
func buildOpenchainRESTRouter() *web.Router {
	s := serverREST{}
	router = web.New(s)
	router.Middleware((*serverREST).SetResponseType)
	router.Get("/getaddr", (*serverREST).GetAddr)
	router.Get("/querymsg/:addr", (*serverREST).QueryMsg)
	router.Post("/sendtxmsg", (*serverREST).SendTxMsg)
	router.Get("/blockheight", (*serverREST).BlockHeight)
	router.Get("/block/:blkNum", (*serverREST).Block)
	router.Get("/querytx/:txHash", (*serverREST).TxHash)

	return router
}

func StartRESTServer() {
	router := buildOpenchainRESTRouter()
	port := strconv.FormatUint(uint64(config.Config.RestPort), 10)

	RestLog = logging.GetLogIns()
	RestLog.Info(LOGTABLE_REST, "rest start port: "+port)

	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		panic(err)
	}
}
