package cli

import(
	"github.com/urfave/cli"
	"net/http"
	"io/ioutil"
	"fmt"
	"bytes"
	"encoding/json"
)


var (
	CliRest = cli.StringFlag{
		Name: "restport",
		Value: "6100",
		Usage: "restport",
	}
	CliSendMsg = cli.StringFlag{
		Name: "sendmsg",
		Value: "hello everybody",
		Usage: "sendmsg",
	}

)

var netMsg = cli.Command{
	Name:      "sendmsg",
	Usage:     "sendmsg usage",
	Action:    SendMsg,
	Flags : []cli.Flag {
		CliSendMsg,
		CliRest,
	},
}

func SendMsg(ctx *cli.Context){
	port := ctx.String(CliRest.Name)

	fmt.Println("port:" + port)

	msg := ctx.String(CliSendMsg.Name)
	fmt.Println("msg:" + msg)

	m,_ := json.Marshal(msg)
	response, err := http.Post("http://"+ "127.0.0.1:" + port + "/sendmsg", "application/json", bytes.NewReader(m))

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

var netC = cli.Command{
	Name:      "getaddr",
	Usage:     "getaddr usage",
	Action:    GetAddr,
	Flags : []cli.Flag {
		CliRest,
	},
}

func GetAddr(ctx *cli.Context){
	port := ctx.String(CliRest.Name)

	fmt.Println("port:" + port)
	response, err := http.Get("http://"+ "127.0.0.1:"+ port + "/getaddr")
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