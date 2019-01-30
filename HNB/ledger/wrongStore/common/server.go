package common

type WrongIndexStore interface {
	GetWrongIndex(txid []byte) (string, error)
	SetWrongIndex(txid []byte, reason string) error
}
