package util

import (
	"sync/atomic"
)

type AtomicNo struct {
	No uint64
}

func NewAtomicNo(No uint64) *AtomicNo {
	return &AtomicNo{
		No: No,
	}
}

func (a *AtomicNo) GetCurrentNo() uint64 {
	return atomic.LoadUint64(&a.No)
}

func (a *AtomicNo) GetNextNo() uint64 {
	return atomic.AddUint64(&a.No, 1)
}

func (a *AtomicNo) SetNo(No uint64) {
	atomic.StoreUint64(&a.No, No)
}
