package cli

import (
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/txpool"
	"github.com/urfave/cli"
)

var (
	CliTxPoolLen = cli.StringFlag{
		Name:  "chaindID",
		Value: "HGS",
		Usage: "chainid",
	}
)

var ReadTxCount = cli.Command{
	Name:   "txpoollen",
	Usage:  "read txpool len",
	Action: GetTxcount,
	Flags: []cli.Flag{
		CliTxPoolLen,
	},
}

func GetTxcount(ctx *cli.Context) error {
	chainID := ctx.String(CliRest.Name)
	txCount := txpool.TxsLen(chainID)
	fmt.Printf(">> the %s txpool len %d\n", chainID, txCount)
	return nil
}
