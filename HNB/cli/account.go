package cli

import (
	"HNB/cli/common"
	"HNB/cli/utils"
	"HNB/msp"
	"bufio"
	"fmt"
	"github.com/urfave/cli"
	"strings"
)

type curveInfo struct {
	name string
	code byte
}

type schemeInfo struct {
	name string
	code int
}

var schemeMap = map[string]schemeInfo{
	"":                {"SHA256withECDSA", msp.ECDSAP256},
	"1":               {"SHA256withECDSA", msp.ECDSAP256},
	"SHA256withECDSA": {"SHA256withECDSA", msp.ECDSAP256},
}

func chooseScheme(reader *bufio.Reader) string {
	common.PrintNotice("signature-scheme")
	for true {
		tmp, _ := reader.ReadString('\n')
		tmp = strings.TrimSpace(tmp)

		_, ok := schemeMap[tmp]
		if ok {
			fmt.Printf("scheme %s is selected.\n", schemeMap[tmp].name)
			return schemeMap[tmp].name
		} else {
			fmt.Print("Input error! Please enter a number above:")
		}
	}
	return ""
}

//func checkFileName(ctx *cli.Context) string {
//	if ctx.IsSet(utils.GetFlagName(utils.FileFlag)) {
//		return ctx.String(utils.GetFlagName(utils.FileFlag))
//	} else {
//		//default account file name
//		return DEFAULT_WALLET_FILE_NAME
//	}
//}

func checkFileName(ctx *cli.Context) string {
	if ctx.IsSet(utils.GetFlagName(utils.FileFlag)) {
		return ctx.String(utils.GetFlagName(utils.FileFlag))
	} else {
		//default account file name
		return DEFAULT_KEYPAIR_FILE_NAME
	}
}

func checkScheme(ctx *cli.Context, reader *bufio.Reader) string {
	sch := ""
	sigFlag := utils.GetFlagName(utils.SigSchemeFlag)

	if ctx.IsSet(sigFlag) {
		if _, ok := schemeMap[ctx.String(sigFlag)]; ok {
			sch = schemeMap[ctx.String(sigFlag)].name
			fmt.Printf("%s is selected. \n", sch)
		} else {
			fmt.Printf("%s is not a valid content for option -s \n", ctx.String(sigFlag))
			sch = chooseScheme(reader)
		}
	} else {
		sch = chooseScheme(reader)
	}

	return sch
}

