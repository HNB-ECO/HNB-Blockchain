

package cli

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"HNB/account"
	"github.com/urfave/cli"
	"os"
)

var (
	AccountCommand = cli.Command{
		Action:    cli.ShowSubcommandHelp,
		Name:      "account",
		Usage:     "Manage accounts",
		ArgsUsage: "[arguments...]",
		Description: `Wallet management commands can be used to add, view, modify, delete, import account, and so on.
You can use ./HNB account --help command to view help information of wallet management command.`,
		Subcommands: []cli.Command{
			{
				Action:    accountCreate,
				Name:      "add",
				Usage:     "Add a new account",
				ArgsUsage: "[sub-command options]",
				Flags: []cli.Flag{
					utils.AccountQuantityFlag,
					utils.AccountTypeFlag,
					utils.AccountKeylenFlag,
					utils.AccountSigSchemeFlag,
					utils.AccountDefaultFlag,
					utils.AccountLabelFlag,
					utils.WalletFileFlag,
				},
				Description: ` Add a new account to wallet.
   HNB support three type of key: ecdsa, sm2 and ed25519, and support 224、256、384、521 bits length of key in ecdsa, but only support 256 bits length of key in sm2 and ed25519.
   HNB support multiple signature scheme.`,
			},
			{
				Action:    accountList,
				Name:      "list",
				Usage:     "List existing accounts",
				ArgsUsage: "[sub-command options] <label|address|index>",
				Flags: []cli.Flag{
					utils.WalletFileFlag,
					utils.AccountVerboseFlag,
				},
				Description: `List existing accounts. If specified in args, will list those account. If not specified in args, will list all accouns in wallet`,
			},
			{
				Action:    accountSet,
				Name:      "set",
				Usage:     "Modify an account",
				ArgsUsage: "[sub-command options] <label|address|index>",
				Flags: []cli.Flag{
					utils.AccountSetDefaultFlag,
					utils.WalletFileFlag,
					utils.AccountLabelFlag,
					utils.AccountChangePasswdFlag,
					utils.AccountSigSchemeFlag,
				},
				Description: `Modify settings for an account. Account is specified by address, label of index. Index start from 1. This can be showed by the 'list' command.`,
			},
			{
				Action:    accountDelete,
				Name:      "del",
				Usage:     "Delete an account",
				ArgsUsage: "[sub-command options] <address|label|index>",
				Flags: []cli.Flag{
					utils.WalletFileFlag,
				},
				Description: `Delete an account specified by address, label of index. Index start from 1. This can be showed by the 'list' command`,
			},
			{
				Action:    accountImport,
				Name:      "import",
				Usage:     "Import accounts of wallet to another",
				ArgsUsage: "[sub-command options] <address|label|index>",
				Flags: []cli.Flag{
					utils.WalletFileFlag,
					utils.AccountSourceFileFlag,
					utils.AccountWIFFlag,
				},
				Description: "Import accounts of wallet to another. If not specific accounts in args, all account in source will be import",
			},
			{
				Action:    accountExport,
				Name:      "export",
				Usage:     "Export accounts to a specified wallet file",
				ArgsUsage: "[sub-command options] <filename>",
				Flags: []cli.Flag{
					utils.WalletFileFlag,
					utils.AccountLowSecurityFlag,
				},
			},
		},
	}
)

func accountCreate(ctx *cli.Context) error {
	reader := bufio.NewReader(os.Stdin)
	optionType := ""
	optionCurve := ""
	optionScheme := ""

	optionDefault := ctx.IsSet(utils.GetFlagName(utils.AccountDefaultFlag))
	if !optionDefault {
		optionType = checkType(ctx, reader)
		optionCurve = checkCurve(ctx, reader, &optionType)
		optionScheme = checkScheme(ctx, reader, &optionType)
	} else {
		fmt.Printf("Use default setting '-t ecdsa -b 256 -s SHA256withECDSA' \n")
		fmt.Printf("	signature algorithm: %s \n", keyTypeMap[optionType].name)
		fmt.Printf("	curve: %s \n", curveMap[optionCurve].name)
		fmt.Printf("	signature scheme: %s \n", schemeMap[optionScheme].name)
	}
	optionFile := checkFileName(ctx)
	optionNumber := checkNumber(ctx)
	optionLabel := checkLabel(ctx)
	pass, _ := password.GetConfirmedPassword()
	keyType := keyTypeMap[optionType].code
	curve := curveMap[optionCurve].code
	scheme := schemeMap[optionScheme].code
	wallet, err := account.Open(optionFile)
	if err != nil {
		return fmt.Errorf("Open wallet error:%s", err)
	}
	defer common.ClearPasswd(pass)
	for i := 0; i < optionNumber; i++ {
		label := optionLabel
		if label != "" && optionNumber > 1 {
			label = fmt.Sprintf("%s%d", label, i+1)
		}
		acc, err := wallet.NewAccount(label, keyType, curve, scheme, pass)
		if err != nil {
			return fmt.Errorf("new account error:%s", err)
		}
		fmt.Println()
		fmt.Println("Index:", wallet.GetAccountNum())
		fmt.Println("Label:", label)
		fmt.Println("Address:", acc.Address.ToBase58())
		fmt.Println("Public key:", hex.EncodeToString(keypair.SerializePublicKey(acc.PublicKey)))
		fmt.Println("Signature scheme:", acc.SigScheme.Name())
	}

	fmt.Println("\nCreate account successfully.")
	return nil
}

func accountList(ctx *cli.Context) error {
	optionFile := checkFileName(ctx)
	wallet, err := account.Open(optionFile)
	if err != nil {
		return fmt.Errorf("Open wallet:%s error:%s", optionFile, err)
	}
	accNum := wallet.GetAccountNum()
	if accNum == 0 {
		fmt.Println("No account")
		return nil
	}
	accList := make(map[string]string, ctx.NArg())
	for i := 0; i < ctx.NArg(); i++ {
		addr := ctx.Args().Get(i)
		accMeta := common.GetAccountMetadataMulti(wallet, addr)
		if accMeta == nil {
			fmt.Printf("Cannot find account by:%s in wallet:%s\n", addr, utils.GetFlagName(utils.WalletFileFlag))
			continue
		}
		accList[accMeta.Address] = ""
	}
	for i := 1; i <= accNum; i++ {
		accMeta := wallet.GetAccountMetadataByIndex(i)
		if accMeta == nil {
			continue
		}
		if len(accList) > 0 {
			_, ok := accList[accMeta.Address]
			if !ok {
				continue
			}
		}
		if !ctx.Bool(utils.GetFlagName(utils.AccountVerboseFlag)) {
			if accMeta.IsDefault {
				fmt.Printf("Index:%-4d Address:%s  Label:%s (default)\n", i, accMeta.Address, accMeta.Label)
			} else {
				fmt.Printf("Index:%-4d Address:%s  Label:%s\n", i, accMeta.Address, accMeta.Label)
			}
			continue
		}
		if accMeta.IsDefault {
			fmt.Printf("%v\t%v (default)\n", i, accMeta.Address)
		} else {
			fmt.Printf("%v\t%v\n", i, accMeta.Address)
		}
		fmt.Printf("	Label: %v\n", accMeta.Label)
		fmt.Printf("	Signature algorithm: %v\n", accMeta.KeyType)
		fmt.Printf("	Curve: %v\n", accMeta.Curve)
		fmt.Printf("	Key length: %v bits\n", len(accMeta.Key)*8)
		fmt.Printf("	Public key: %v\n", accMeta.PubKey)
		fmt.Printf("	Signature scheme: %v\n", accMeta.SigSch)
		fmt.Println()
	}
	return nil
}

