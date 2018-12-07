package hgs

import (
	appComm "HNB/appMgr/common"
	"HNB/logging"
	"encoding/json"
	"github.com/pkg/errors"
	"strconv"
	"HNB/util"
	"bytes"
	"HNB/txpool"
)

var HgsLog logging.LogModule

const (
	LOGTABLE_HGS string = "hgs"
)

type hsgTx struct {
	OutputAddr []byte `json:"output"`
	Amount     int64  `json:"amount"`
	//1  same   2  diff
	TxType     uint8  `json:"txType"`
	//only TxType = 2 valid
	InAmount int64    `json:"inAmount"`
}

type hgs struct {
}

const(
	SAME = 1
	DIFF = 2
)

var user = []string{"68175250cadc6f2524f55e891e244116fec4b690"}

func GetContractHandler() (appComm.ContractInf, error) {
	HgsLog = logging.GetLogIns()
	HgsLog.Info(LOGTABLE_HGS, "Invoke Init func")
	return &hgs{}, nil
}

func InstallContract(ca appComm.ContractApi) error {
	HgsLog = logging.GetLogIns()
	HgsLog.Info(LOGTABLE_HGS, "Install Contract")
	amountS := strconv.FormatInt(10000000, 10)

	//TODO 从配置文件初始化账户余额
	for _, v := range user {
		ca.PutState(util.HexToByte(v), []byte(amountS))
	}

	return nil
}

func (h *hgs) Init() error {
	return nil
}

func (h *hgs)SamePro(ca appComm.ContractApi, hx *hsgTx) error{

	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	if bytes.Compare(from, hx.OutputAddr) == 0{
		return errors.New("in and out addr same")
	}

	input, _ := ca.GetState(from)
	if input == nil {
		return errors.New("input not exist")
	}

	inputAmount, _ := strconv.ParseInt(string(input), 10, 64)

	if inputAmount < hx.Amount {
		return errors.New("insufficient balance")
	}

	var bAmount int64 = 0
	//addr := common.Address{}
	//addr.SetBytes(hx.OutputAddr)
	output, _ := ca.GetState(hx.OutputAddr)
	if output != nil {
		bAmount, _ = strconv.ParseInt(string(output), 10, 64)
	}

	a := strconv.FormatInt(inputAmount - hx.Amount, 10)
	b := strconv.FormatInt(bAmount + hx.Amount, 10)
	ca.PutState(from, []byte(a))
	ca.PutState(hx.OutputAddr, []byte(b))

	return nil
}
func (h *hgs)DiffPro(ca appComm.ContractApi, hx *hsgTx) error{
	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	//if bytes.Compare(from, hx.OutputAddr) == 0{
	//	return errors.New("in and out addr same")
	//}

	input, _ := ca.GetState(from)
	if input == nil {
		return errors.New("input not exist")
	}

	inputAmount, _ := strconv.ParseInt(string(input), 10, 64)

	if inputAmount < hx.InAmount {
		return errors.New("insufficient balance")
	}

	var bAmount int64 = 0

	//from other coin
	output, _ := ca.GetOtherState(txpool.HNB, hx.OutputAddr)
	//output, _ := ca.GetState(hx.OutputAddr)
	if output != nil {
		bAmount, _ = strconv.ParseInt(string(output), 10, 64)
	}

	a := strconv.FormatInt(inputAmount - hx.InAmount, 10)
	b := strconv.FormatInt(bAmount + hx.Amount, 10)
	ca.PutState(from, []byte(a))

	//from other coin
	ca.PutOtherState(txpool.HNB, hx.OutputAddr, []byte(b))

	return nil
}

func (h *hgs) Invoke(ca appComm.ContractApi) error {
	HgsLog.Debugf(LOGTABLE_HGS, "Invoke args: %v", ca.GetArgs())

	arg := ca.GetArgs()
	hx := hsgTx{}
	err := json.Unmarshal([]byte(arg), &hx)
	if err != nil {
		return err
	}

	if hx.TxType == SAME{
		return h.SamePro(ca, &hx)
	}else{
		return h.DiffPro(ca, &hx)
	}


	return nil
}

func (h *hgs) Query(ca appComm.ContractApi) ([]byte, error) {
	HgsLog.Infof(LOGTABLE_HGS, "Query args:%v", ca.GetArgs())
	addr := ca.GetArgs()
	value, _ := ca.GetState(addr)
	return value, nil
}

