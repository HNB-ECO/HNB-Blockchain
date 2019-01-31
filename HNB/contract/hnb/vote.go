package hnb

import (
	appComm "HNB/appMgr/common"
	"HNB/msp"
	"encoding/json"
	"github.com/pkg/errors"
	"sort"
	"strconv"
)

type VoteInfo struct {
	FromAddr    []byte `json:"fromAddr"`
	Candidate   []byte `json:"candidate"`
	VotingPower int64  `json:""`
	VoteEpoch   uint64 `json:"voteEpoch"`
}
type PeerInfo struct {
	VotingPower int64
	PeerID      []byte
}

type VoteRecord struct {
	FromAddr    []byte
	VotingPower int64
}

type PeersIDSet struct {
	PeerIDs [][]byte
}

type peerInfo []PeerInfo

func (s peerInfo) Len() int           { return len(s) }
func (s peerInfo) Less(i, j int) bool { return s[i].VotingPower > s[j].VotingPower }
func (s peerInfo) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// 需要支持的功能
// 根据轮次统计投票和 降序排列
// 投票总数
// 根据轮次解冻质押的token
// 存储结构 Key:"epoch#$epoch" Value:"降序排列 map key:{peerID:xxx} value:{token:xxx}"
// 存储结构 key:"epochInfo#$epoch" json 数组 {FromAddr, Amount}

func (h *hnb) VotePro(ca appComm.ContractApi, vi *VoteInfo) error {
	err := h.SaveTokenTotal(ca, vi)
	if err != nil {
		return err
	}

	err = h.SaveTokenFromInfo(ca, vi)
	if err != nil {
		return err
	}

	return nil
}

func (h *hnb) SaveTokenTotal(ca appComm.ContractApi, vi *VoteInfo) error {

	if vi.Candidate == nil || vi.FromAddr == nil || vi.VotingPower == 0 || vi.VoteEpoch == 0 {
		return errors.New("vote params invalid")
	}
	input, err := ca.GetState(vi.FromAddr)
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

	if inputAmount < vi.VotingPower {
		return errors.New("vote power lower")
	}

	voteRecord, err := ca.GetState(GetEpochKey(vi.VoteEpoch))
	if err != nil {
		return nil
	}

	voteRecordMap := make(map[string]int64, 0)
	if voteRecord != nil {
		err = json.Unmarshal(voteRecord, &voteRecordMap)
		if err != nil {
			return err
		}
	}

	key := msp.PeerIDToString(vi.Candidate)

	voteAmount, ok := voteRecordMap[key]
	if !ok {
		voteAmount = 0
	}

	voteAmount += vi.VotingPower

	voteRecordMap[key] = voteAmount

	vote, _ := json.Marshal(voteRecordMap)

	err = ca.PutState(GetEpochKey(vi.VoteEpoch), vote)
	if err != nil {
		return err
	}
	b := strconv.FormatInt(inputAmount-vi.VotingPower, 10)
	err = ca.PutState(vi.FromAddr, []byte(b))
	if err != nil {
		return err
	}
	HnbLog.Debugf(LOGTABLE_HNB, "vote pro:%v", string(vote))

	return nil
}

func (h *hnb) SaveTokenFromInfo(ca appComm.ContractApi, vi *VoteInfo) error {

	voteRecord, err := ca.GetState(GetEpochRecordKey(vi.VoteEpoch))
	if err != nil {
		return nil
	}

	vrs := make([]*VoteRecord, 0)

	if voteRecord != nil {
		err = json.Unmarshal(voteRecord, &vrs)
		if err != nil {
			return err
		}
	}

	vr := &VoteRecord{}
	vr.VotingPower = vi.VotingPower
	vr.FromAddr = vi.FromAddr

	vrs = append(vrs, vr)
	vrsm, _ := json.Marshal(vrs)

	err = ca.PutState(GetEpochRecordKey(vi.VoteEpoch), vrsm)
	if err != nil {
		return err
	}

	return nil
}

func (h *hnb) GetVoteTokenOrder(ca appComm.ContractApi, epochNo uint64) (*PeersIDSet, error) {

	voteRecord, err := ca.GetState(GetEpochKey(epochNo))
	if err != nil {
		return nil, nil
	}

	voteRecordMap := make(map[string]int64, 0)
	if voteRecord != nil {
		err = json.Unmarshal(voteRecord, &voteRecordMap)
		if err != nil {
			return nil, err
		}
	}
	var pis []PeerInfo
	for k, v := range voteRecordMap {
		pi := PeerInfo{}
		pi.VotingPower = v
		pi.PeerID = msp.StringToByteKey(k)
		pis = append(pis, pi)
	}

	sort.Sort(peerInfo(pis))

	res := make([][]byte, 0)
	for _, v := range pis {
		res = append(res, v.PeerID)
	}
	return &PeersIDSet{res}, nil
}

func (h *hnb) GetVoteSum(ca appComm.ContractApi, epochNo uint64) (int64, error) {
	voteRecord, err := ca.GetState(GetEpochKey(epochNo))
	if err != nil {
		return 0, nil
	}

	voteRecordMap := make(map[string]int64, 0)
	if voteRecord != nil {
		err = json.Unmarshal(voteRecord, &voteRecordMap)
		if err != nil {
			return 0, err
		}
	}

	var sum int64 = 0
	for _, v := range voteRecordMap {
		sum += v
	}
	return sum, nil
}

func (h *hnb) UnFreezePro(ca appComm.ContractApi, epochNo uint64) error {
	voteRecord, err := ca.GetState(GetEpochRecordKey(epochNo))
	if err != nil {
		return nil
	}

	vrs := make([]*VoteRecord, 0)
	if voteRecord != nil {
		err = json.Unmarshal(voteRecord, &vrs)
		if err != nil {
			return err
		}
	}

	for _, v := range vrs {
		amount, err := h.GetBalance(ca, v.FromAddr)
		if err != nil {
			return err
		}
		err = h.SetBalance(ca, v.FromAddr, amount+v.VotingPower)
		if err != nil {
			return err
		}
	}

	err = ca.DelState(GetEpochRecordKey(epochNo))
	if err != nil {
		return err
	}

	err = ca.DelState(GetEpochKey(epochNo))
	if err != nil {
		return err
	}
	return nil
}
