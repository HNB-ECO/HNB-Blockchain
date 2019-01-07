package cli

import (
	"fmt"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
)

var (
	CliRest = cli.StringFlag{
		Name:  "restport",
		Value: "6100",
		Usage: "SyncPort number where Hnb interacts with the client",
	}
)

var GetNetworkAddr = cli.Command{
	Name:   "getaddr",
	Usage:  "Display other node information linked by this node. Currently, this field only displays active information.",
	Action: GetAddr,
	Flags: []cli.Flag{
		CliRest,
	},
}

func GetAddr(ctx *cli.Context) {
	port := ctx.String(CliRest.Name)

	fmt.Println("port:" + port)
	response, err := http.Get("http://" + "127.0.0.1:" + port + "/getaddr")
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

