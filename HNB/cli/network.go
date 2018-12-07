package cli

import (
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
		Usage: "Port number where Hnb interacts with the client",
	}
	CliSendMsg = cli.StringFlag{
		Name:  "sendmsg",
		Value: "Hello World",
		Usage: "Send the cli interface of the transaction",
	}
	CliQueryMsg = cli.StringFlag{
		Name:  "addr",
		Value: "admin",
		Usage: "Query account balance",
	}
)

var netMsg = cli.Command{
	Name:   "sendmsg",
	Usage:  "Send the cli interface of the transaction",
	Action: SendMsg,
	Flags: []cli.Flag{
		CliSendMsg,
		CliRest,
	},
}

var QueryMsgC = cli.Command{
	Name:   "querymsg",
	Usage:  "Query account balance",
	Action: QueryMsg,
	Flags: []cli.Flag{
		CliQueryMsg,
		CliRest,
	},
}

var SendMsgC = cli.Command{
	Name:   "sendmsg",
	Usage:  "Send the cli interface of the transaction",
	Action: SendMsg,
	Flags: []cli.Flag{
		CliSendMsg,
		CliRest,
	},
}

func QueryMsg(ctx *cli.Context) {
	port := ctx.String(CliRest.Name)

	fmt.Println("port:" + port)

	msg := ctx.String(CliQueryMsg.Name)
	fmt.Println("addr:" + msg)

	url := "http://" + "127.0.0.1:" + port + "/querymsg/" + msg

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
}
func SendMsg(ctx *cli.Context) {
	port := ctx.String(CliRest.Name)

	fmt.Println("port:" + port)

	msg := ctx.String(CliSendMsg.Name)
	fmt.Println("msg:" + msg)

	m, _ := json.Marshal(msg)

	url := "http://" + "127.0.0.1:" + port + "/sendtxmsg"

	if url != "" {
		response, err := http.Post(url, "application/json", bytes.NewReader(m))

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

var netC = cli.Command{
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

