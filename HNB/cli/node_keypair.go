package cli

import (
	"HNB/bccsp"
	"HNB/bccsp/secp256k1"
	"HNB/bccsp/sw"
	"HNB/cli/utils"
	"HNB/msp"
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"github.com/urfave/cli"
	"math/big"
	"os"
)

const (
	DEFAULT_KEYPAIR_FILE_NAME = "./node.key"
)

var (
	KeypairCommand = cli.Command{
		Action:      cli.ShowSubcommandHelp,
		Name:        "nodekeypair",
		Usage:       "Manage nodekeypair",
		ArgsUsage:   "[arguments...]",
		Description: `nodekeypair create keypair and store`,
		Subcommands: []cli.Command{
			{
				Action:    keypairCreate,
				Name:      "add",
				Usage:     "Add a new nide keypair",
				ArgsUsage: "[sub-command options]",
				Flags: []cli.Flag{
					utils.SigSchemeFlag,
					utils.DefaultFlag,
					utils.FileFlag,
				},
			},
			{
				Action:    readPubKey,
				Name:      "read",
				Usage:     "read a new node keypair",
				ArgsUsage: "[sub-command options]",
				Flags: []cli.Flag{
					utils.FileFlag,
				},
			},
		},
	}
)

func readPubKey(ctx *cli.Context) error {
	filePath := checkFileName(ctx)
	sKeyPair, err := msp.Load(filePath)
	if err != nil {
		return err
	}

	var keyO bccsp.Key

	algType := sKeyPair.Scheme
	switch algType {
	case msp.ECDSAP256:
		key := new(ecdsa.PrivateKey)
		key.Curve = secp256k1.S256()
		key.D = new(big.Int)
		key.D.SetBytes(sKeyPair.PriKey)
		key.PublicKey.X, key.PublicKey.Y = elliptic.Unmarshal(secp256k1.S256(), sKeyPair.PubKey)

		keyO = &sw.Ecdsa256K1PublicKey{&key.PublicKey}

	default:
		fmt.Printf("algType not support : %v\n", algType)
		return fmt.Errorf("algType not support")
	}

	keyStr := msp.BccspKeyToString(sKeyPair.Scheme, keyO)
	address := msp.AccountPubkeyToAddress1(keyO)

	fmt.Printf("pubkey:%s, address:%x\n", keyStr, address)
	return nil
}

func keypairCreate(ctx *cli.Context) error {
	reader := bufio.NewReader(os.Stdin)
	optionScheme := ""

	optionDefault := ctx.IsSet(utils.GetFlagName(utils.DefaultFlag))
	if !optionDefault {
		optionScheme = checkScheme(ctx, reader)
	} else {
		fmt.Printf("Use default setting '-t ecdsa -b 256 -s SHA256withECDSA' \n")
		fmt.Printf("	signature scheme: %s \n", schemeMap[optionScheme].name)
	}
	optionPath := checkFileName(ctx)
	scheme := schemeMap[optionScheme].code

	keyPair, err := NewKeyPair(scheme)
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("new node keypair error:%s", err)
	}
	err = msp.Save(keyPair, optionPath)

	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("\nCreate node keypair successfully.")
	return nil
}

func NewKeyPair(scheme int) (*msp.KeyPair, error) {
	prvkey, err := msp.GeneratePriKey(scheme)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("generateKeyPair error:%s", err)
	}
	pubkey, err := prvkey.PublicKey()
	if err != nil {
		return nil, err
	}
	keyPair := &msp.KeyPair{
		Scheme: scheme,
		PriKey: prvkey,
		PubKey: pubkey,
	}
	return keyPair, nil
}
