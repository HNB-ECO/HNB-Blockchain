package common

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/common"
)

const (
	HGS string = "hgs"
	HNB string = "hnb"
)

type ContractInf interface {
	//	InstallContract(ContractApi) error

	Init() error
	Invoke(ContractApi) error
	Query(ContractApi) ([]byte, error)
	CheckTx(ContractApi) error
}

type ContractApi interface {
	PutState(key, value []byte) error
	GetState(key []byte) ([]byte, error)
	DelState(key []byte) error
	GetArgs() []byte
	SetFrom(address common.Address)
	GetFrom() common.Address
	GetOtherState(chainID string, key []byte) ([]byte, error)
	PutOtherState(chainID string, key, value []byte) error
}
