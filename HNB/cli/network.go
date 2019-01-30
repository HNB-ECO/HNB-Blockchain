package cli

import (
	"HNB/access/rest"
	"bytes"
	"encoding/json"
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
	url := "http://" + "127.0.0.1:" + port + "/"
	jm := &rest.JsonrpcMessage{Version: "1.0"}
	jm.Method = "getAddr"
	jmm, _ := json.Marshal(jm)

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

}
