package common

import "fmt"

func PrintNotice(name string) {
	switch name {
	case "key type":
		fmt.Printf(`
Select a signature algorithm from the following:

  1  ECDSA
  2  SM2
  3  Ed25519

[default is 1]: `)
		break

	case "curve":
		fmt.Printf(`
Select a curve from the following:

    | NAME  | KEY LENGTH (bits)
 ---|-------|------------------
  1 | P-224 | 224
  2 | P-256 | 256
  3 | P-384 | 384
  4 | P-521 | 521

This determines the length of the private key [default is 2]: `)
		break

	case "signature-scheme":
		fmt.Printf(`
Select a signature scheme from the following:

  1  SHA256withECDSA

This can be changed later [default is 1]: `)
		break

	default:
	}
}

