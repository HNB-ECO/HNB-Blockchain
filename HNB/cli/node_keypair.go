package cli

import (
	"github.com/urfave/cli"
	"bufio"
	"os"
	"HNB/cli/utils"
	"fmt"
	"encoding/json"
	"HNB/msp"
	"HNB/cli/common"
	"io/ioutil"
	bccspUtils "HNB/bccsp/utils"
	//"HNB/bccsp/sw"
)

const (
	DEFAULT_KEYPAIR_FILE_NAME = "./node.key"
)




var (
	NodeKeypairCommand = cli.Command{
		Action:    cli.ShowSubcommandHelp,
		Name:      "nodekeypair",
		Usage:     "Manage nodekeypair",
		ArgsUsage: "[arguments...]",
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

func readPubKey(ctx *cli.Context) error{
	filePath := checkFileName(ctx)
	key,err := msp.Load(filePath)
	if err != nil{
		return err
	}
	sPriKey, err := bccspUtils.PEMtoPrivateKey(key.PriKey, nil)
	if err != nil {
		return err
	}
	sk, err := msp.BuildPriKey(key.Scheme, sPriKey)
	if err != nil {
		return err
	}

	keyO, err := sk.PublicKey()
	if err != nil {
		return err
	}

	keyStr := msp.GetPubStrFromO(key.Scheme, keyO)
	fmt.Println(keyStr)
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
	err = save(keyPair, optionPath)

	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println("\nCreate node keypair successfully.")
	return nil
}

func NewKeyPair(scheme int) (*msp.KeyPair, error) {
	fmt.Println(0)
	prvkey, err := msp.GeneratePriKey(scheme)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("generateKeyPair error:%s", err)
	}
	fmt.Println(1)
	pubkey, err  := prvkey.PublicKey()
	if err != nil {
		return nil ,err
	}
	fmt.Println(1)
	keyPair := &msp.KeyPair{
		Scheme:scheme,
		PriKey:prvkey,
		PubKey:pubkey,
	}
	return keyPair, nil
}

func save(keypair *msp.KeyPair, path string) error {
	priKey , err := msp.GetPriKey(keypair.Scheme, keypair.PriKey)
	if err != nil {
		return err
	}
	sPriKey, err := bccspUtils.PrivateKeyToPEM(priKey, nil)
	if err != nil {
		return err
	}
	pubKey , err := msp.GetPubKey(keypair.Scheme, keypair.PubKey)
	if err != nil {
		return err
	}
	sPubKey, err := bccspUtils.PublicKeyToPEM(pubKey, nil)
	if err != nil {
		return err
	}
	saveKP := &msp.SaveKeyPair{
		Scheme:keypair.Scheme,
		PubKey:sPubKey,
		PriKey:sPriKey,
	}
	data, err := json.Marshal(saveKP)
	if err != nil {
		return err
	}
	if common.FileExisted(path) {
		filename := path + "~"
		err := ioutil.WriteFile(filename, data, 0644)
		if err != nil {
			return err
		}
		return os.Rename(filename, path)
	} else {
		return ioutil.WriteFile(path, data, 0644)
	}
	//keyStore, err := sw.NewFileBasedKeyStore(nil, path, false)
	//if err != nil {
	//	return err
	//}
	//err = keyStore.StoreKey(keypair.PriKey)
	//if err != nil {
	//	return err
	//}
	//if common.FileExisted(idPath) {
	//	filename := idPath + "~"
	//	err := ioutil.WriteFile(filename, keypair.PriKey.SKI(), 0644)
	//	if err != nil {
	//		return err
	//	}
	//	return os.Rename(filename, idPath)
	//} else {
	//	return ioutil.WriteFile(idPath, keypair.PriKey.SKI(), 0644)
	//}
}

