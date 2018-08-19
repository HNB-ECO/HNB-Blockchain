package appMgr

import (
	"HNB/ledger"
	ssComm "HNB/ledger/stateStore/common"
)

type contractApi struct{
	args string
	sc *StateCache
	chainID string
}

func GetContractApi(chainID string) *contractApi{
	sc := NewStateCache()
	return &contractApi{sc:sc, chainID:chainID}
}

func (s *contractApi)IsDelState(key []byte) bool{
	v := s.sc.Get(s.chainID, key)
	if v != nil && v.State == ssComm.Deleted{
		return true
	}

	return false
}

func (s *contractApi)GetAllState() []*ssComm.StateItem{
	return s.sc.Find()
}

func (s *contractApi)PutState(key, value []byte) error{
	s.sc.Put(s.chainID, key, value)
	return nil
}

func (s *contractApi)GetState(key []byte) ([]byte, error){
	if s.IsDelState(key) == true{
		return nil, nil
	}
	v := s.sc.Get(s.chainID, key)
	if v == nil{
		return ledger.GetContractState(s.chainID, key)
	}

	return v.Value, nil
}

func (s *contractApi)DelState(key []byte) error{
	s.sc.Delete(s.chainID, key)
	return nil
}

func (s *contractApi)GetStringArgs() string{
	return s.args
}

func (s *contractApi)SetStringArgs(args string){
	s.args = args
}

