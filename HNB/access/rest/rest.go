package rest

import (
	"net/http"
	"github.com/gocraft/web"
	"HNB/config"
	"strconv"
	"HNB/p2pNetwork"
	"encoding/json"
	"fmt"
	"io/ioutil"
)

var router *web.Router


type serverREST struct {
}

func (s *serverREST) SetResponseType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json")

	// Enable CORS
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "accept, content-type")

	next(rw, req)
}
func (*serverREST) GetAddr(rw web.ResponseWriter, req *web.Request){
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	peers := p2pNetwork.P2PIns.GetNeighborAddrs()
	encoder.Encode(peers)
}

func (*serverREST) SendMsg(rw web.ResponseWriter, req *web.Request){
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	reqBody, _ := ioutil.ReadAll(req.Body)
	var msg string
	err := json.Unmarshal(reqBody, &msg)
	if err != nil{
		encoder.Encode(err.Error())
		return
	}
	p2pNetwork.P2PIns.Xmit(msg, true)
	encoder.Encode("ok")
}

// 构建router & 向router中注册url
func buildOpenchainRESTRouter() *web.Router {
	s := serverREST{}
	router = web.New(s)
	router.Middleware((*serverREST).SetResponseType)
	router.Get("/getaddr", (*serverREST).GetAddr)
	router.Post("/sendmsg", (*serverREST).SendMsg)

	return router
}



func StartRESTServer() {
	router := buildOpenchainRESTRouter()
	port := strconv.FormatUint(uint64(config.Config.RestPort), 10)
	fmt.Println(":" + port)
	err := http.ListenAndServe(":" + port, router)
	if err != nil {
		panic(err)
	}
}

