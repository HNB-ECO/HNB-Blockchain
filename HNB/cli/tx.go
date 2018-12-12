package cli

import (
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
	Usage:  "Get the number of transactions in the tx pool, including two parts queue and pending",
	Action: GetTxcount,
	Flags: []cli.Flag{
		CliTxPoolLen,
	},
}

func GetTxcount(ctx *cli.Context) error {
	//chainID := ctx.String(CliRest.Name)
	//txCount := txpool.TxsLen(chainID)
	//fmt.Printf(">> the %s txpool len %d\n", chainID, txCount)
	return nil
}

