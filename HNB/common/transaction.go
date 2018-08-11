package common

type Transaction struct {
	Type 	  	string				//交易类型
	Payload   	string 				//具体交易的数据序列化后的
	Txid      	string 				//交易的
	Timestamp 	uint64            	//时间戳
	Signature 	[]byte				//签名
}

func NewTransaction() *Transaction {
	return new(Transaction)
}
