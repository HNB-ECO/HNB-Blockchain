package appMgr

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	ssComm "github.com/HNB-ECO/HNB-Blockchain/HNB/ledger/stateStore/common"
)

type contractApi struct {
	args    []byte
	sc      *StateCache
	chainID string
	from    common.Address
}

func GetContractApi(chainID string) *contractApi {
	sc := NewStateCache()
	return &contractApi{sc: sc, chainID: chainID}
}

func (s *contractApi) GetSnapshot() map[string]*ssComm.StateItem {
	return s.sc.GetChangeSet()
}

func (s *contractApi) SnapshotRestore(si map[string]*ssComm.StateItem) {
	s.sc.Restore(si)
}

func (s *contractApi) IsDelState(key []byte) bool {
	v := s.sc.Get(s.chainID, key)
	if v != nil && v.State == ssComm.Deleted {
		return true
	}

	return false
}

func (s *contractApi) isDelState(chainID string, key []byte) bool {
	v := s.sc.Get(chainID, key)
	if v != nil && v.State == ssComm.Deleted {
		return true
	}

	return false
}

func (s *contractApi) GetOtherState(chainID string, key []byte) ([]byte, error) {
	if s.isDelState(chainID, key) == true {
		return nil, nil
	}
	v := s.sc.Get(chainID, key)
	if v == nil {
		return ledger.GetContractState(chainID, key)
	}

	return v.Value, nil
}

func (s *contractApi) PutOtherState(chainID string, key, value []byte) error {

	s.sc.Put(chainID, key, value)
	return nil
}

func (s *contractApi) Merge(src []*ssComm.StateItem) {
	for _, v := range src {
		state := v.State
		switch state {
		case ssComm.Changed:
			s.PutOtherState(v.ChainID, v.Key, v.Value)
		case ssComm.Deleted:
			s.DelState(v.Key)
		}
	}
}

func (s *contractApi) GetAllState() []*ssComm.StateItem {
	return s.sc.Find()
}

func (s *contractApi) PutState(key, value []byte) error {
	s.sc.Put(s.chainID, key, value)
	return nil
}

func (s *contractApi) GetState(key []byte) ([]byte, error) {
	if s.IsDelState(key) == true {
		return nil, nil
	}
	v := s.sc.Get(s.chainID, key)
	if v == nil {
		return ledger.GetContractState(s.chainID, key)
	}

	return v.Value, nil
}

func (s *contractApi) DelState(key []byte) error {

	s.sc.Delete(s.chainID, key)
	return nil
}

func (s *contractApi) GetArgs() []byte {
	return s.args
}

func (s *contractApi) SetArgs(args []byte) {
	s.args = args
}

func (s *contractApi) SetFrom(address common.Address) {
	s.from = address
}

func (s *contractApi) GetFrom() common.Address {
	return s.from
}
