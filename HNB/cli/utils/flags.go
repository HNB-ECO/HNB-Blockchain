package utils

import (
	"github.com/urfave/cli"
	"strings"
)

var (
	TypeFlag = cli.StringFlag{
		Name:  "type,t",
		Usage: "Specifies the `<key-type>` by signature algorithm.",
	}
	KeylenFlag = cli.StringFlag{
		Name:  "bit-length,b",
		Usage: "Specifies the `<bit-length>` of key",
	}
	SigSchemeFlag = cli.StringFlag{
		Name:  "signature-scheme,s",
		Usage: "Specifies the signature scheme `<scheme>`",
	}
	DefaultFlag = cli.BoolFlag{
		Name:  "default,d",
		Usage: "Use default settings to create a new account (equal to '-t ecdsa -b 256 -s SHA256withECDSA')",
	}
	FileFlag = cli.StringFlag{
		Name:  "node,w",
		Usage: "Use `<filename>` as the node keypair",
	}
)

func GetFlagName(flag cli.Flag) string {
	name := flag.GetName()
	if name == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(name, ",")[0])
}
