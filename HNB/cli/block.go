package cli

import (
	"fmt"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
	"bytes"
	"HNB/access/rest"
	"encoding/json"
)

var (
	CliBlkNum = cli.StringFlag{
		Name:  "blkNum",
		Value: "0",
		Usage: "Hnb supports querying intra-block transaction data based on block number",
	}

	CliTxHash = cli.StringFlag{
		Name:  "txHash",
		Value: "0",
		Usage: "Hnb supports trading data based on query hashes based on transactions",
	}
)

//read height
var ReadBlkHeight = cli.Command{
	Name:   "blockheight",
	Usage:  "Hnb supports querying the current block height, block height = highest block number +1",
	Action: ReadHeight,
	Flags: []cli.Flag{
		CliRest,
	},
}

func ReadHeight(ctx *cli.Context) error {

	port := ctx.String(CliRest.Name)
	url := "http://" + "127.0.0.1:" + port + "/"
	jm := &rest.JsonrpcMessage{Version:"1.0"}
	jm.Method = "blockNumber"
	jm.ID,_ = json.Marshal(1)
	jmm,_ := json.Marshal(jm)
	if url != "" {
		response, err := http.Post(url, "application/json", bytes.NewReader(jmm))

		if err != nil {
			fmt.Println(err)
		}
		result, _ := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(">>" + string(result))
		response.Body.Close()
	}
	return nil
}

var ReadBlkByNum = cli.Command{
	Name:   "block",
	Usage:  "Hnb supports querying intra-block transaction data based on block number",
	Action: ReadBlkNum,
	Flags: []cli.Flag{
		CliRest,
		CliBlkNum,
	},
}


func ReadBlkNum(ctx *cli.Context) error {

	blkNum := ctx.String(CliBlkNum.Name)
	port := ctx.String(CliRest.Name)
	url := "http://" + "127.0.0.1:" + port + "/"
	jm := &rest.JsonrpcMessage{Version:"1.0"}
	jm.Method = "getBlockByNumber"
	var params []interface{}
	params = append(params, blkNum)
	jm.Params,_ = json.Marshal(params)
	jmm,_ := json.Marshal(jm)
	if url != "" {
		response, err := http.Post(url, "application/json", bytes.NewReader(jmm))

		if err != nil {
			fmt.Println(err)
		}
		result, _ := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(">>" + string(result))
		response.Body.Close()
	}
	return nil
}

var ReadTxByHash = cli.Command{
	Name:   "qryTxHash",
	Usage:  "Hnb supports trading data based on query hashes based on transactions",
	Action: QueryTxHash,
	Flags: []cli.Flag{
		CliRest,
		CliTxHash,
	},
}

func QueryTxHash(ctx *cli.Context) error {

	hash := ctx.String(CliTxHash.Name)
	port := ctx.String(CliRest.Name)

	url := "http://" + "127.0.0.1:" + port + "/"
	jm := &rest.JsonrpcMessage{Version:"1.0"}
	jm.Method = "getBlockByHash"
	var params []interface{}
	params = append(params, hash)
	jm.Params,_ = json.Marshal(params)
	jmm,_ := json.Marshal(jm)

	if url != "" {
		response, err := http.Post(url, "application/json", bytes.NewReader(jmm))

		if err != nil {
			fmt.Println(err)
		}
		result, _ := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(">>" + string(result))
		response.Body.Close()
	}

	return nil
}

