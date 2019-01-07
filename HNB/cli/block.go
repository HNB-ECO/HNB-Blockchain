package cli

import (
	"fmt"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
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
	url := "http://" + "127.0.0.1:" + port + "/blockheight"

	if url != "" {
		response, err := http.Get(url)

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
	url := "http://" + "127.0.0.1:" + port + "/block/" + blkNum

	if url != "" {
		response, err := http.Get(url)

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
	url := "http://" + "127.0.0.1:" + port + "/querytx/" + hash

	if url != "" {
		response, err := http.Get(url)

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

