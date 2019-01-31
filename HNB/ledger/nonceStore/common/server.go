package common

import "HNB/common"

type NonceSet struct {
	NI []*NonceItem
}

type NonceItem struct {
	Key   common.Address
	Nonce uint64
}

type NonceStore interface {
	GetNonce(address common.Address) (uint64, error)
	SetNonce(address common.Address, nonce uint64) error
}
