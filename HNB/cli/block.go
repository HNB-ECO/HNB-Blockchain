package cli

import(
	"github.com/urfave/cli"
	"net/http"
	"io/ioutil"
	"fmt"
)

var (
	CliBlkNum = cli.StringFlag{
		Name: "blkNum",
		Value: "0",
		Usage: "blk num",
	}
)
//read height
var ReadBlkC = cli.Command{
	Name:      "blockheight",
	Usage:     "read block height",
	Action:    ReadHeight,
	Flags : []cli.Flag {
		CliRest,
	},
}

func ReadHeight(ctx *cli.Context) error {

	port := ctx.String(CliRest.Name)
	url := "http://"+ "127.0.0.1:" + port + "/blockheight"

	if url != ""{
		response, err := http.Get(url)

		if err != nil{
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


var ReadBlkNumC = cli.Command{
	Name:      "block",
	Usage:     "read block info",
	Action:    ReadBlkNum,
	Flags : []cli.Flag {
		CliRest,
		CliBlkNum,
	},
}

func ReadBlkNum(ctx *cli.Context) error {

	blkNum := ctx.String(CliBlkNum.Name)
	port := ctx.String(CliRest.Name)
	url := "http://"+ "127.0.0.1:" + port + "/block/" + blkNum

	if url != ""{
		response, err := http.Get(url)

		if err != nil{
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
