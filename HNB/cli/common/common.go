
package common

import (
	"fmt"
	"github.com/urfave/cli"
	"strconv"
	"os"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/account"
)

func GetPasswd(ctx *cli.Context) ([]byte, error) {
	var passwd []byte
	var err error
	if ctx.IsSet(utils.GetFlagName(utils.AccountPassFlag)) {
		passwd = []byte(ctx.String(utils.GetFlagName(utils.AccountPassFlag)))
	} else {
		passwd, err = password.GetAccountPassword()
		if err != nil {
			return nil, fmt.Errorf("Input password error:%s", err)
		}
	}
	return passwd, nil
}

func OpenWallet(ctx *cli.Context) (account.Client, error) {
	walletFile := ctx.String(utils.GetFlagName(utils.WalletFileFlag))
	if walletFile == "" {
		walletFile = config.DEFAULT_WALLET_FILE_NAME
	}
	if !common.FileExisted(walletFile) {
		return nil, fmt.Errorf("cannot find wallet file:%s", walletFile)
	}
	wallet, err := account.Open(walletFile)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

func GetAccountMulti(wallet account.Client, passwd []byte, accAddr string) (*account.Account, error) {
	if accAddr == "" {
		defAcc, err := wallet.GetDefaultAccount(passwd)
		if err != nil {
			return nil, err
		}

		fmt.Printf("Using default account:%s\n", defAcc.Address.ToBase58())
		return defAcc, nil
	}
	acc, err := wallet.GetAccountByAddress(accAddr, passwd)
	if err != nil {
		return nil, fmt.Errorf("getAccountByAddress:%s error:%s", accAddr, err)
	}
	if acc != nil {
		return acc, nil
	}
	acc, err = wallet.GetAccountByLabel(accAddr, passwd)
	if err != nil {
		return nil, fmt.Errorf("getAccountByLabel:%s error:%s", accAddr, err)
	}
	if acc != nil {
		return acc, nil
	}
	index, err := strconv.ParseInt(accAddr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("cannot get account by:%s", accAddr)
	}
	acc, err = wallet.GetAccountByIndex(int(index), passwd)
	if err != nil {
		return nil, fmt.Errorf("getAccountByIndex:%d error:%s", index, err)
	}
	if acc != nil {
		return acc, nil
	}
	return nil, fmt.Errorf("cannot get account by:%s", accAddr)
}

func GetAccountMetadataMulti(wallet account.Client, accAddr string) *account.AccountMetadata {

	if accAddr == "" {
		fmt.Printf("Using default account:%s\n", accAddr)
		return wallet.GetDefaultAccountMetadata()
	}
	acc := wallet.GetAccountMetadataByAddress(accAddr)
	if acc != nil {
		return acc
	}
	acc = wallet.GetAccountMetadataByLabel(accAddr)
	if acc != nil {
		return acc
	}
	index, err := strconv.ParseInt(accAddr, 10, 32)
	if err != nil {
		return nil
	}
	return wallet.GetAccountMetadataByIndex(int(index))
}

func GetAccount(ctx *cli.Context, address ...string) (*account.Account, error) {
	wallet, err := OpenWallet(ctx)
	if err != nil {
		return nil, err
	}
	passwd, err := GetPasswd(ctx)
	if err != nil {
		return nil, err
	}
	defer ClearPasswd(passwd)
	accAddr := ""
	if len(address) > 0 {
		accAddr = address[0]
	} else {
		accAddr = ctx.String(utils.GetFlagName(utils.AccountAddressFlag))
	}
	return GetAccountMulti(wallet, passwd, accAddr)
}

func IsBase58Address(address string) bool {
	if address == "" {
		return false
	}
	_, err := common.AddressFromBase58(address)
	return err == nil
}

func ParseAddress(address string, ctx *cli.Context) (string, error) {
	if IsBase58Address(address) {
		return address, nil
	}
	wallet, err := OpenWallet(ctx)
	if err != nil {
		return "", err
	}
	acc := wallet.GetAccountMetadataByLabel(address)
	if acc != nil {
		return acc.Address, nil
	}
	index, err := strconv.ParseInt(address, 10, 32)
	if err != nil {
		return "", fmt.Errorf("cannot get account by:%s", address)
	}
	acc = wallet.GetAccountMetadataByIndex(int(index))
	if acc != nil {
		return acc.Address, nil
	}
	return "", fmt.Errorf("cannot get account by:%s", address)
}

func ClearPasswd(passwd []byte) {
	size := len(passwd)
	for i := 0; i < size; i++ {
		passwd[i] = 0
	}
}

func FileExisted(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

