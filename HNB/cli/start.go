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
	"HNB/ledger"
	tp "HNB/txpool"
	"HNB/appMgr"
	"HNB/consensus"
	"HNB/db"
	"HNB/msp"
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
	CliConfigEnabledCons = cli.StringFlag{
		Name: "enabledCons",
		Value: "true",
		Usage: "enabled",
	}
	CliKeypairPath = cli.StringFlag{
		Name: "keypairPath",
		Value: "./node.key",
		Usage: "keypair path",
	}
)


func Init(){
	app := cli.NewApp()
	app.Action = Start
	app.Name = "111"
	app.ArgsUsage = "222"
	app.Description = "333"
	app.Version = "1.0.0"
	app.Author = "HGS Developer"
	app.HelpName = "444"
	app.UsageText = "555"
	app.Usage = "666"

	app.Flags = []cli.Flag {
		CliLogPath,
		CliConfigPath,
		CliLogLevel,
		CliConfigEnabledCons,
		CliKeypairPath,
	}

	app.Commands = []cli.Command{
		netC,
		SendMsgC,
		QueryMsgC,
		NodeKeypairCommand,
		ReadBlkC,
		ReadBlkNumC,
		ReadTxCount,
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

	if ctx.IsSet(CliConfigEnabledCons.Name){
		enabledCons := ctx.GlobalBool(CliConfigEnabledCons.Name)
		config.Config.EnableConsensus = enabledCons
	}



	if ctx.IsSet(CliKeypairPath.Name) {
		keyPairPath := ctx.GlobalString(CliKeypairPath.Name)
		config.Config.KetPairPath = keyPairPath
	}


	fmt.Printf("logPath=%s logLevel=%s\n enableCons=%v\n keyPairPath=%v\n",
		config.Config.Log.Path,
			config.Config.Log.Level,
				config.Config.EnableConsensus,
					config.Config.KetPairPath)



	logging.InitLogModule()


	err := msp.NewKeyPair().Init(config.Config.KetPairPath)
	if err != nil {
		panic("msp init err: "+err.Error())
	}

	db, err := db.InitDB("leveldb")
	if err != nil{
		panic(err.Error())
	}
	ledger.InitLedger(db)
	//如果为了效率的提升，可以直接访问，无须消息总线
	//msgBus.InitMsgBus()
	//
	//msgBus.Subscribe("111", Add)
	//a := 1
	//b := 2
	//msgBus.Publish("111",&a, &b)

	appMgr.InitAppMgr(db)

	err = p2pNetwork.NewServer().Start()
	if err != nil{
		panic("network err: " + err.Error())
	}

	tp.NewTXPoolServer().InitTXPoolServer()

	consensus.NewConsensusServer("algorand", tp.HGS).Start()

	rest.StartRESTServer()
	for{
		time.Sleep(time.Hour)
	}
}
