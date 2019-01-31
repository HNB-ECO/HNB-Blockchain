package common

import (
	"sync"
	"sync/atomic"
)

type AtomicIndex struct {
	index uint64
}

func NewAtomicIndex(index uint64) *AtomicIndex {
	return &AtomicIndex{
		index: index,
	}
}

func (a *AtomicIndex) GetCurrentIndex() uint64 {
	return atomic.LoadUint64(&a.index)
}

func (a *AtomicIndex) GetNextIndex() uint64 {
	return atomic.AddUint64(&a.index, 1)
}

func (a *AtomicIndex) SetIndex(index uint64) {
	atomic.StoreUint64(&a.index, index)
}

type MutexBool struct {
	sync.RWMutex
	value bool
}

func (m *MutexBool) SetTrue() {
	m.Lock()
	m.value = true
	m.Unlock()
}
func (m *MutexBool) SetFalse() {
	m.Lock()
	m.value = false
	m.Unlock()
}

func (m *MutexBool) Get() bool {
	var b bool
	m.RLock()
	b = m.value
	m.RUnlock()
	return b
}
