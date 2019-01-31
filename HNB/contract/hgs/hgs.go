package hgs

import (
	appComm "HNB/appMgr/common"
	"HNB/logging"
	"HNB/util"
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
)

var HgsLog logging.LogModule

const (
	LOGTABLE_HGS string = "hgs"
)

type SameTx struct {
	OutputAddr util.HexBytes `json:"output"`
	Amount     int64         `json:"amount"`
}

type DiffTx struct {
	OutputAddr util.HexBytes `json:"output"`
	Amount     int64         `json:"amount"`
	InAmount   int64         `json:"inAmount"`
}

type HgsTx struct {
	//1  same   2  diff  3 vote
	TxType  uint8  `json:"txType"`
	PayLoad []byte `json:"payLoad"`
}

type QryHgsTx struct {
	//1  balance   2  ....
	TxType  uint8  `json:"txType"`
	PayLoad []byte `json:"payLoad"`
}

type hgs struct {
}

const (
	SAME = 1
	DIFF = 2
)

const (
	BALANCE = 1
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
		err := ca.PutState(util.HexToByte(v), []byte(amountS))
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *hgs) Init() error {
	return nil
}

func (h *hgs) CheckSamePro(ca appComm.ContractApi, hx *SameTx) error {
	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	if bytes.Compare(from, hx.OutputAddr) == 0 {
		return errors.New("in and out addr same")
	}

	input, err := ca.GetState(from)
	if err != nil {
		return err
	}
	if input == nil {
		return errors.New("input not exist")
	}

	inputAmount, err := strconv.ParseInt(string(input), 10, 64)
	if err != nil {
		return err
	}
	if inputAmount < hx.Amount {
		return errors.New("insufficient balance")
	}

	return nil
}

func (h *hgs) SamePro(ca appComm.ContractApi, hx *SameTx) error {

	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	if bytes.Compare(from, hx.OutputAddr) == 0 {
		return errors.New("in and out addr same")
	}

	input, err := ca.GetState(from)
	if err != nil {
		return err
	}
	if input == nil {
		return errors.New("input not exist")
	}

	inputAmount, err := strconv.ParseInt(string(input), 10, 64)
	if err != nil {
		return err
	}
	if inputAmount < hx.Amount {
		return errors.New("insufficient balance")
	}

	var bAmount int64 = 0
	//addr := common.Address{}
	//addr.SetBytes(hx.OutputAddr)
	output, err := ca.GetState(hx.OutputAddr)
	if err != nil {
		return err
	}
	if output != nil {
		bAmount, _ = strconv.ParseInt(string(output), 10, 64)
	}

	a := strconv.FormatInt(inputAmount-hx.Amount, 10)
	b := strconv.FormatInt(bAmount+hx.Amount, 10)
	err = ca.PutState(from, []byte(a))
	if err != nil {
		return err
	}
	err = ca.PutState(hx.OutputAddr, []byte(b))
	if err != nil {
		return err
	}
	return nil
}

func (h *hgs) CheckDiffPro(ca appComm.ContractApi, hx *DiffTx) error {
	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	//if bytes.Compare(from, hx.OutputAddr) == 0{
	//	return errors.New("in and out addr same")
	//}

	input, err := ca.GetState(from)
	if err != nil {
		return err
	}
	if input == nil {
		return errors.New("input not exist")
	}

	inputAmount, err := strconv.ParseInt(string(input), 10, 64)
	if err != nil {
		return err
	}
	if inputAmount < hx.InAmount {
		return errors.New("insufficient balance")
	}
	return nil
}

func (h *hgs) DiffPro(ca appComm.ContractApi, hx *DiffTx) error {
	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	//if bytes.Compare(from, hx.OutputAddr) == 0{
	//	return errors.New("in and out addr same")
	//}

	input, err := ca.GetState(from)
	if err != nil {
		return err
	}
	if input == nil {
		return errors.New("input not exist")
	}

	inputAmount, err := strconv.ParseInt(string(input), 10, 64)
	if err != nil {
		return err
	}
	if inputAmount < hx.InAmount {
		return errors.New("insufficient balance")
	}

	var bAmount int64 = 0

	//from other coin
	output, err := ca.GetOtherState(appComm.HNB, hx.OutputAddr)
	if err != nil {
		return err
	}
	//output, _ := ca.GetState(hx.OutputAddr)
	if output != nil {
		bAmount, err = strconv.ParseInt(string(output), 10, 64)
		if err != nil {
			return err
		}
	}

	a := strconv.FormatInt(inputAmount-hx.InAmount, 10)
	b := strconv.FormatInt(bAmount+hx.Amount, 10)
	err = ca.PutState(from, []byte(a))
	if err != nil {
		return err
	}
	//from other coin
	err = ca.PutOtherState(appComm.HNB, hx.OutputAddr, []byte(b))
	if err != nil {
		return err
	}
	return nil
}

func (h *hgs) CheckTx(ca appComm.ContractApi) error {
	HgsLog.Debugf(LOGTABLE_HGS, "Check tx args: %v", ca.GetArgs())

	arg := ca.GetArgs()
	hx := HgsTx{}
	err := json.Unmarshal([]byte(arg), &hx)
	if err != nil {
		return err
	}

	switch hx.TxType {
	case SAME:
		smTx := SameTx{}
		err = json.Unmarshal(hx.PayLoad, &smTx)
		if err != nil {
			return err
		}
		return h.CheckSamePro(ca, &smTx)
	case DIFF:
		dfTx := DiffTx{}
		err = json.Unmarshal(hx.PayLoad, &dfTx)
		if err != nil {
			return err
		}
		return h.CheckDiffPro(ca, &dfTx)
	}
	return nil
}

func (h *hgs) Invoke(ca appComm.ContractApi) error {
	HgsLog.Debugf(LOGTABLE_HGS, "Invoke args: %v", ca.GetArgs())

	arg := ca.GetArgs()
	hx := HgsTx{}
	err := json.Unmarshal([]byte(arg), &hx)
	if err != nil {
		return err
	}

	switch hx.TxType {
	case SAME:
		smTx := SameTx{}
		err = json.Unmarshal(hx.PayLoad, &smTx)
		if err != nil {
			return err
		}
		return h.SamePro(ca, &smTx)
	case DIFF:
		dfTx := DiffTx{}
		err = json.Unmarshal(hx.PayLoad, &dfTx)
		if err != nil {
			return err
		}
		return h.DiffPro(ca, &dfTx)
	}

	return nil
}

func (h *hgs) Query(ca appComm.ContractApi) ([]byte, error) {
	HgsLog.Infof(LOGTABLE_HGS, "Query args:%v", ca.GetArgs())
	msg := ca.GetArgs()
	qh := QryHgsTx{}
	err := json.Unmarshal(msg, &qh)
	if err != nil {
		return nil, err
	}
	switch qh.TxType {
	case BALANCE:
		value, err := ca.GetState(qh.PayLoad)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	return nil, nil
}
