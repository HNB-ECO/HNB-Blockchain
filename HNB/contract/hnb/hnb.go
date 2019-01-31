package hnb

import (
	appComm "HNB/appMgr/common"
	"HNB/logging"
	"HNB/util"
	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"strconv"
)

var HnbLog logging.LogModule

const (
	LOGTABLE_HNB string = "hnb"
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

type HnbTx struct {
	//1  same   2  diff  3 vote
	TxType  uint8  `json:"txType"`
	PayLoad []byte `json:"payLoad"`
}

type QryHnbTx struct {
	//1  balance   2  ....
	TxType  uint8  `json:"txType"`
	PayLoad []byte `json:"payLoad"`
}

type hnb struct {
}

const (
	SAME uint8 = iota + 1
	DIFF
	POS_VOTE_TRANSCATION
	UNFREEZE_TOKEN
)

const (
	TOTAL_BALANCE        = 10000000
	VOTEDB        string = "epoch"
	VOTEDB_RECORD        = "record_epoch"
)

var (
	ErrInPutNotExist = errors.New("input not exist")
)

const (
	BALANCE uint8 = iota + 1
	VOTESUM
	TOKENORDER
)

var user = []string{"68175250cadc6f2524f55e891e244116fec4b690"}

func GetContractHandler() (appComm.ContractInf, error) {
	HnbLog = logging.GetLogIns()
	HnbLog.Info(LOGTABLE_HNB, "Invoke Init func")
	return &hnb{}, nil
}

func InstallContract(ca appComm.ContractApi) error {
	HnbLog = logging.GetLogIns()
	HnbLog.Info(LOGTABLE_HNB, "Install Contract")
	amountS := strconv.FormatInt(TOTAL_BALANCE, 10)

	//TODO 从配置文件初始化账户余额
	for _, v := range user {
		err := ca.PutState(util.HexToByte(v), []byte(amountS))
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *hnb) Init() error {
	return nil
}

func (h *hnb) CheckSamePro(ca appComm.ContractApi, hx *SameTx) error {

	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	if bytes.Compare(from, hx.OutputAddr) == 0 {
		return errors.New("in and out addr same")
	}

	inputAmount, err := h.GetBalance(ca, from)
	if err != nil {
		return err
	}

	if inputAmount < hx.Amount {
		return errors.New("insufficient balance")
	}

	return nil
}

func (h *hnb) CheckDiffPro(ca appComm.ContractApi, hx *DiffTx) error {
	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	inputAmount, err := h.GetBalance(ca, from)
	if err != nil {
		return err
	}

	if inputAmount < hx.InAmount {
		return errors.New("insufficient balance")
	}

	return nil
}

func (h *hnb) SamePro(ca appComm.ContractApi, hx *SameTx) error {

	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	if bytes.Compare(from, hx.OutputAddr) == 0 {
		return errors.New("in and out addr same")
	}

	inputAmount, err := h.GetBalance(ca, from)
	if err != nil {
		return err
	}

	if inputAmount < hx.Amount {
		return errors.New("insufficient balance")
	}

	var bAmount int64 = 0

	bAmount, err = h.GetBalance(ca, hx.OutputAddr)
	if err != nil {
		return err
	}

	err = h.SetBalance(ca, from, inputAmount-hx.Amount)
	if err != nil {
		return err
	}

	err = h.SetBalance(ca, hx.OutputAddr, bAmount+hx.Amount)
	if err != nil {
		return err
	}
	return nil
}
func (h *hnb) DiffPro(ca appComm.ContractApi, hx *DiffTx) error {
	fromAddr := ca.GetFrom()
	from := fromAddr.GetBytes()

	inputAmount, err := h.GetBalance(ca, from)
	if err != nil {
		return err
	}

	if inputAmount < hx.InAmount {
		return errors.New("insufficient balance")
	}

	var bAmount int64 = 0

	//from other coin
	bAmount, err = h.GetOtherStateBalance(ca, appComm.HGS, hx.OutputAddr)
	if err != nil {
		return err
	}

	err = h.SetBalance(ca, from, inputAmount-hx.InAmount)
	if err != nil {
		return err
	}

	err = h.SetOtherStateBalance(ca, appComm.HGS, hx.OutputAddr, bAmount+hx.Amount)
	if err != nil {
		return err
	}

	return nil
}

func (h *hnb) SetBalance(ca appComm.ContractApi, addr []byte, amount int64) error {
	b := strconv.FormatInt(amount, 10)
	err := ca.PutState(addr, []byte(b))
	return err
}

func (h *hnb) GetBalance(ca appComm.ContractApi, addr []byte) (int64, error) {
	input, err := ca.GetState(addr)
	if err != nil {
		return 0, err
	}

	if input == nil {
		return 0, nil
	}

	amount, err := strconv.ParseInt(string(input), 10, 64)
	if err != nil {
		return 0, err
	}

	return amount, nil
}

func (h *hnb) SetOtherStateBalance(ca appComm.ContractApi, chainID string, addr []byte, amount int64) error {
	b := strconv.FormatInt(amount, 10)
	err := ca.PutOtherState(chainID, addr, []byte(b))
	return err
}

func (h *hnb) GetOtherStateBalance(ca appComm.ContractApi, chainID string, addr []byte) (int64, error) {
	input, err := ca.GetOtherState(chainID, addr)
	if err != nil {
		return 0, err
	}

	if input == nil {
		return 0, nil
	}

	amount, err := strconv.ParseInt(string(input), 10, 64)
	if err != nil {
		return 0, err
	}

	return amount, nil
}

func GetEpochKey(epoch uint64) []byte {
	es := strconv.FormatUint(epoch, 10)
	return []byte(VOTEDB + "#" + es)
}

func GetEpochRecordKey(epoch uint64) []byte {
	es := strconv.FormatUint(epoch, 10)
	return []byte(VOTEDB_RECORD + "#" + es)
}

func (h *hnb) Invoke(ca appComm.ContractApi) error {
	HnbLog.Debugf(LOGTABLE_HNB, "Invoke args: %v", ca.GetArgs())
	arg := ca.GetArgs()
	hx := HnbTx{}
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
	case POS_VOTE_TRANSCATION:
		posVote := VoteInfo{}
		err = json.Unmarshal(hx.PayLoad, &posVote)
		if err != nil {
			return err
		}
		return h.VotePro(ca, &posVote)
	case UNFREEZE_TOKEN:
		epochNo, err := strconv.ParseUint(string(hx.PayLoad), 10, 64)
		if err != nil {
			return err
		}
		return h.UnFreezePro(ca, epochNo)
	}

	return nil
}

func (h *hnb) CheckTx(ca appComm.ContractApi) error {
	HnbLog.Debugf(LOGTABLE_HNB, "CheckTx args: %v", ca.GetArgs())
	arg := ca.GetArgs()
	hx := HnbTx{}
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
	case POS_VOTE_TRANSCATION:
		//posVote := VoteInfo{}
		//err = json.Unmarshal(hx.PayLoad, &posVote)
		//if err != nil {
		//	return err
		//}
		//return h.CheckVotePro(ca, &posVote)
	case UNFREEZE_TOKEN:
		epochNo, err := strconv.ParseUint(string(hx.PayLoad), 10, 64)
		if err != nil {
			return err
		}
		return h.UnFreezePro(ca, epochNo)
	}

	return nil
}

func (h *hnb) Query(ca appComm.ContractApi) ([]byte, error) {
	HnbLog.Infof(LOGTABLE_HNB, "Query args:%v", ca.GetArgs())
	msg := ca.GetArgs()
	qh := QryHnbTx{}
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
	case VOTESUM:
		epochNo, err := strconv.ParseUint(string(qh.PayLoad), 10, 64)
		if err != nil {
			return nil, err
		}
		sum, err := h.GetVoteSum(ca, epochNo)
		if err != nil {
			return nil, err
		}
		sumS := strconv.FormatInt(sum, 10)
		return []byte(sumS), nil
	case TOKENORDER:
		epochNo, err := strconv.ParseUint(string(qh.PayLoad), 10, 64)
		if err != nil {
			return nil, err
		}
		peerIDs, err := h.GetVoteTokenOrder(ca, epochNo)
		if err != nil {
			return nil, err
		}
		m, _ := json.Marshal(peerIDs)
		return m, nil
	}
	return nil, nil
}
