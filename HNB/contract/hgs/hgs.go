package hgs

import (
	appComm "HNB/appMgr/common"
	"HNB/logging"
	"encoding/json"
	"strconv"
	"errors"
)


var HgsLog logging.LogModule
const(
	LOGTABLE_HGS string = "hgs"
)

type hsgTx struct{
	InputAddr []byte `json:"input"`
	OutputAddr []byte `json:"output"`
	Amount int64 `json:"amount"`
	Signature []byte `json:"signature"`
}

type hgs struct {

}




func GetContractHandler() (appComm.ContractInf, error){
	HgsLog = logging.GetLogIns()
	HgsLog.Info(LOGTABLE_HGS, "Invoke Init func")
	return &hgs{}, nil
}

func InstallContract(ca appComm.ContractApi) error{
	HgsLog = logging.GetLogIns()
	HgsLog.Info(LOGTABLE_HGS, "Install Contract")
	amountS := strconv.FormatInt(10000, 10)
	amountS1 := strconv.FormatInt(20000, 10)
	ca.PutState([]byte("zhangsan"), []byte(amountS))
	ca.PutState([]byte("lisi"), []byte(amountS1))
	return nil
}

func (h *hgs)Init() error{
	return nil
}


func (h *hgs)Invoke(ca appComm.ContractApi) error{
	HgsLog.Info(LOGTABLE_HGS, "Invoke args:" + ca.GetStringArgs())
	arg := ca.GetStringArgs()
	hx := hsgTx{}
	err := json.Unmarshal([]byte(arg), &hx)
	if err != nil{
		return err
	}

	input, _ := ca.GetState(hx.InputAddr)
	if input == nil{
		return errors.New("input not exist")
	}

	inputAmount,_ := strconv.ParseInt(string(input), 10 ,64)

	if inputAmount < hx.Amount{
		return errors.New("amount less")
	}

	var bAmount int64 = 0
	output, _ := ca.GetState(hx.OutputAddr)
	if output != nil{
		bAmount,_ = strconv.ParseInt(string(output), 10 ,64)
	}

	a := strconv.FormatInt(inputAmount - hx.Amount, 10)
	b := strconv.FormatInt(bAmount + hx.Amount, 10)
	ca.PutState([]byte(hx.InputAddr), []byte(a))
	ca.PutState([]byte(hx.OutputAddr), []byte(b))

	return nil
}

func (h *hgs)Query(ca appComm.ContractApi) ([]byte, error){
	HgsLog.Info(LOGTABLE_HGS, "Query args:" + ca.GetStringArgs())

	addr := ca.GetStringArgs()
	value,_ := ca.GetState([]byte(addr))
	return value, nil
}
