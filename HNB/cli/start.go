package cli


import (
	"os"
	"github.com/urfave/cli"
	"fmt"
	"HNB/config"
	"HNB/logging"
	"HNB/p2pNetwork"
	"time"
	"HNB/access/rest"
)

var (
	CliLogPath = cli.StringFlag{
		Name: "logPath",
		Value: "./",
		Usage: "log path",
	}
	CliLogLevel = cli.StringFlag{
		Name: "logLevel",
		Value: "info",
		Usage: "log level(info/debug/error/warning)",
	}
	CliConfigPath = cli.StringFlag{
		Name: "configPath",
		Value: "./peer.json",
		Usage: "config path",
	}
)


func Init(){
	app := cli.NewApp()
	app.Action = Start
	app.Name = "111"
	app.ArgsUsage = "222"
	app.Description = "333"
	app.Version = "1.0.0"
	app.Author = "HNB Developer"
	app.HelpName = "444"
	app.UsageText = "555"
	app.Usage = "666"

	app.Flags = []cli.Flag {
		CliLogPath,
		CliConfigPath,
		CliLogLevel,
	}

	app.Commands = []cli.Command{
		netC,
		netMsg,
	}

	app.Run(os.Args)
}

func Start(ctx *cli.Context){
	configFile := ctx.GlobalString(CliConfigPath.Name)

	config.LoadConfig(configFile)

	if ctx.IsSet(CliLogPath.Name){
		logPath := ctx.GlobalString(CliLogPath.Name)
		config.Config.Log.Path = logPath
	}

	if ctx.IsSet(CliLogLevel.Name){
		logLevel := ctx.GlobalString(CliLogLevel.Name)
		config.Config.Log.Level = logLevel
	}


	fmt.Printf("logPath=%s logLevel=%s\n",
		config.Config.Log.Path, config.Config.Log.Level)

	logging.InitLogModule()

	//如果为了效率的提升，可以直接访问，无须消息总线
	//msgBus.InitMsgBus()
	//
	//msgBus.Subscribe("111", Add)
	//a := 1
	//b := 2
	//msgBus.Publish("111",&a, &b)

	err := p2pNetwork.NewServer().Start()
	if err != nil{
		panic("network err: " + err.Error())
	}

	rest.StartRESTServer()
	for{
		time.Sleep(time.Hour)
	}
}