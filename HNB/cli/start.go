package cli

import (
	myrpc "HNB/access/grpc"
	"HNB/access/rest"
	"HNB/appMgr"
	"HNB/config"
	"HNB/consensus"
	"HNB/db"
	"HNB/ledger"
	"HNB/logging"
	"HNB/msp"
	"HNB/p2pNetwork"
	"HNB/sync"
	tp "HNB/txpool"
	"fmt"
	"github.com/urfave/cli"
	"os"
	"time"
)

var (
	CliLogPath = cli.StringFlag{
		Name:  "logPath",
		Value: "./",
		Usage: "Storage path of this node log",
	}
	CliLogLevel = cli.StringFlag{
		Name:  "logLevel",
		Value: "info",
		Usage: "The log levels supported by hnb include: info, debug, error, warning",
	}
	CliConfigPath = cli.StringFlag{
		Name:  "configPath",
		Value: "./peer.json",
		Usage: "The initialization of hnb is inseparable from the configuration file, the path of the configuration file",
	}
	CliConfigEnabledCons = cli.StringFlag{
		Name:  "enabledCons",
		Value: "true",
		Usage: "Node has consensus function",
	}
	CliKeypairPath = cli.StringFlag{
		Name:  "keypairPath",
		Value: "./node.key",
		Usage: "The hnb node identity requires a private key to guarantee the key storage path.",
	}
)

func Init() {
	app := cli.NewApp()
	app.Action = Start
	app.Name = "hnb client"
	app.ArgsUsage = "These parameters are the main functions supported by hnb, and we will continue to enrich and improve according to the subsequent requirements."
	app.Description = "HNB system is designed to enable the founding of sustainable economic model to serve real business entities. The goal is to use HNB system to build a next generation of blockchain-based decentralized economic entity serving an economic ecosystem comprised of over 100 million of consumers and merchants. The HNB economy of scale and its members are ever-increasing as the ecosystem grows."
	app.Version = "1.0.0"
	app.Author = "Hnb users, developers, operators, and people interested in hnb"
	app.HelpName = "If you have any questions, please join WIKI or ask questions in the community."
	app.UsageText = "If you have any questions, please join WIKI or ask questions in the community."

	app.Flags = []cli.Flag{
		CliLogPath,
		CliConfigPath,
		CliLogLevel,
		CliConfigEnabledCons,
		CliKeypairPath,
	}

	app.Commands = []cli.Command{
		GetNetworkAddr,
		SendTx,
		QueryMsgCommand,
		KeypairCommand,
		ReadBlkHeight,
		ReadBlkByNum,
		//ReadTxPoolLen,
		ReadTxByHash,
		SendVoteTx,
	}

	app.Run(os.Args)
}

func Start(ctx *cli.Context) {
	configFile := ctx.GlobalString(CliConfigPath.Name)

	config.LoadConfig(configFile)

	if ctx.IsSet(CliLogPath.Name) {
		logPath := ctx.GlobalString(CliLogPath.Name)
		config.Config.Log.Path = logPath
	}

	if ctx.IsSet(CliLogLevel.Name) {
		logLevel := ctx.GlobalString(CliLogLevel.Name)
		config.Config.Log.Level = logLevel
	}

	if ctx.IsSet(CliConfigEnabledCons.Name) {
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
		panic("msp init err: " + err.Error())
	}

	dbIns, err := db.InitKVDB("leveldb")
	if err != nil {
		panic(err.Error())
	}

	if config.Config.IsBlkSQL == true {
		blkDBIns, err := db.InitSQLDB()
		if err != nil {
			panic(err.Error())
		}
		ledger.InitLedger(dbIns, blkDBIns)
	} else {
		ledger.InitLedger(dbIns, dbIns)
	}
	appMgr.InitAppMgr(dbIns)

	err = p2pNetwork.NewServer().Start()
	if err != nil {
		panic("network err: " + err.Error())
	}

	sync.NewSync().Start()

	tp.NewTXPoolServer().Start()

	//consensus.NewConsensusServer(consensus.ALGORAND, tp.HNB).Start()
	consensus.NewConsensusServer(consensus.DPoS, tp.HNB).Start()

	if config.Config.RestPort != uint16(0) {
		rest.StartRESTServer()
	}

	if config.Config.GPRCPort != uint16(0) {
		myrpc.StartGRPCServer()
	}

	//修改成等待信号结束
	for {
		time.Sleep(time.Hour)
	}
}

