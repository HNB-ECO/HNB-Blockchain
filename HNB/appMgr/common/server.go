package common




type ContractInf interface{
//	InstallContract(ContractApi) error

	Init() error
	Invoke(ContractApi) error
	Query(ContractApi) ([]byte, error)
}



type ContractApi interface{
	PutState(key, value []byte) error
	GetState(key []byte) ([]byte, error)
	DelState(key []byte) error
	GetStringArgs() string
}

