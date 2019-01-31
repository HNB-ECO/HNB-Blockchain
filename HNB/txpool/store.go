package txpool

import (
	"HNB/common"
	"HNB/ledger"
)

type PendingCache struct {
	nonce map[common.Address]uint64
}

func NewPendingNonce() *PendingCache {
	pn := &PendingCache{}
	pn.nonce = make(map[common.Address]uint64)
	return pn
}

func (pc *PendingCache) SetNonce(address common.Address, nonce uint64) error {
	pc.nonce[address] = nonce
	return nil
}

func (pc *PendingCache) GetNonce(address common.Address) uint64 {
	var err error
	nonce, ok := pc.nonce[address]
	if !ok {
		nonce, err = ledger.GetNonce(address)
		if err != nil {
			nonce = 0
		}
	}
	return nonce
}
