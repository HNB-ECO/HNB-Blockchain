package msgHandler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/bftGroup/vrf"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/consensusManager/comm/consensusType"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/msp"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/p2pNetwork"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
	"sort"
	"strings"
	"sync/atomic"
)

func (bftMgr *BftMgr) HandleBgDemand(tdmMsg *cmn.TDMMessage) (*cmn.BftGroupSwitchDemand, error) {
	bgDemand := &cmn.BftGroupSwitchDemand{}
	err := json.Unmarshal(tdmMsg.Payload, bgDemand)
	if err != nil {
		return nil, err
	}

	bgNum := bgDemand.BftGroupNum
	ConsLog.Infof(LOGTABLE_CONS, "receive bgDemand bgNum %d", bgNum)
	bgProposerAddr, err := bftMgr.GetNewBgProposerAddr(bgNum)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(bgProposerAddr, bgDemand.DigestAddr) {
		return nil, fmt.Errorf("(bgDemand) bgProposer Addr %X != %x", bgProposerAddr, bgDemand.DigestAddr)
	}

	if bgNum < bftMgr.CurBftGroup.BgID {
		return nil, fmt.Errorf("(bgDemand) bgPropose bgNum %d != %d", bftMgr.CandidateID, bgNum)
	}

	bgAdviceSlice := bftMgr.GetBgAdviceSlice(bgNum)
	if len(bgAdviceSlice) <= len(bftMgr.TotalValidators.Validators)/3 {
		return nil, fmt.Errorf("(bgDemand) bgPropose bgNum %d not enough bgAdvice, receive %d <= %d", bgNum, len(bgAdviceSlice), len(bftMgr.TotalValidators.Validators)/3)
	}

	if len(bgDemand.BftGroupCandidates) < len(bgDemand.NewBftGroup) {
		return nil, fmt.Errorf("(bgDemand) candidate num %d < newBg Num %d", len(bgDemand.BftGroupCandidates), len(bgDemand.NewBftGroup))
	}

	if len(bgDemand.NewBftGroup) != int(bftMgr.BftNumber) {
		return nil, fmt.Errorf("(bgDemand) newBg num %d != %d", len(bgDemand.NewBftGroup), bftMgr.BftNumber)
	}

	newBg, err := bftMgr.MakeNewBgFromBgCandidateSlice(bgNum, bgDemand.BftGroupCandidates)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "make newBg err %v bgNum %d", err, bgNum)
		return nil, err
	}

	if !bftMgr.NewBgEqualBCE(newBg, bgDemand.NewBftGroup) {
		return nil, fmt.Errorf("(bgDemand) bgPropose newBg %v != %v", newBg, bgDemand.NewBftGroup)
	}

	height, err := ledger.GetBlockHeight()
	if err != nil {
		return nil, err
	}

	currentVRF, err := bftMgr.getCurrentVRF()
	if err != nil {
		return nil, err
	}

	// 验证新的VRFValue和VRFProof
	VRFBgData := &vrf.VRFBgData{
		BgNum:    bgNum,
		BlockNum: height - 1,
		PrevVrf:  currentVRF,
	}

	ConsLog.Debugf(LOGTABLE_CONS, "(bgDemand) vrfBgData bgNum %d blockNum %d preVrf %x", VRFBgData.BgNum, VRFBgData.BlockNum, VRFBgData.PrevVrf)
	index, val := bftMgr.TotalValidators.GetByAddress(bgDemand.DigestAddr)
	if index == -1 || val == nil {
		return nil, fmt.Errorf("(bgDemand) bgNum %d bgProposer %d not found", bgNum, util.HexBytes(bgDemand.DigestAddr))
	}

	pk := msp.StringToBccspKey(val.PubKeyStr)

	verifySuccess, err := vrf.VerifyVRF4bg(pk, VRFBgData, bgDemand.VRFValue, bgDemand.VRFProof, msp.GetAlgType())
	if err != nil {
		return nil, fmt.Errorf("(bgDemand) bgNum %d verifyVRF4bg err %v", bgNum, err)
	}
	if !verifySuccess {
		return nil, fmt.Errorf("(bgDemand) bgNum %d verifyVRF4bg fail VRFValue=%v, VRFProof=%v", bgNum, string(bgDemand.VRFValue), string(bgDemand.VRFProof))
	}

	ConsLog.Infof(LOGTABLE_CONS, "wait got bgDemand bgNum %d <- %v",
		bgDemand.BftGroupNum,
		util.HexBytes(bgDemand.DigestAddr))

	newBftGroup, err := bftMgr.MakeNewBgFromBgAdvice(bgDemand)
	if err != nil {
		return nil, fmt.Errorf("(newBgEle) make newBftGroup err %v bgNum %d", err, bftMgr.CandidateID)
	}

	err = bftMgr.ResetBftGroup(*newBftGroup)
	if err != nil {
		return nil, fmt.Errorf("(newBgEle) reset bft group fail %v bgNum %d", err, bftMgr.CandidateID)
	}

	return bgDemand, nil
}

func (bftMgr *BftMgr) NewBgEqualBCE(newBg1, newBg2 []*cmn.BftGroupSwitchAdvice) bool {
	if len(newBg1) != len(newBg2) {
		return false
	}

	if (newBg1 == nil) != (newBg2 == nil) {
		return false
	}

	newBg2 = newBg2[0:len(newBg1)]
	for i := range newBg1 {
		if !bftMgr.AdviceEqual(newBg1[i], newBg2[i]) {
			return false
		}
	}

	return true
}

func (bftMgr *BftMgr) AdviceEqual(advice1, advice2 *cmn.BftGroupSwitchAdvice) bool {
	if advice1.BftGroupNum != advice2.BftGroupNum {
		return false
	}

	if !bytes.Equal(advice1.DigestAddr, advice2.DigestAddr) {
		return false
	}

	if advice1.Height != advice2.Height {
		return false
	}

	if !bytes.Equal(advice1.Hash, advice2.Hash) {
		return false
	}

	return true
}

func (bftMgr *BftMgr) HandleBgAdviceMsg(tdmMsg *cmn.TDMMessage, senderID uint64) error {
	bgAdvice := &cmn.BftGroupSwitchAdvice{}
	err := json.Unmarshal(tdmMsg.Payload, bgAdvice)
	if err != nil {
		return err
	}

	bgNum := bgAdvice.BftGroupNum

	ConsLog.Infof(LOGTABLE_CONS, "receive bgNum %d advice <- %v", bgNum, util.HexBytes(bgAdvice.DigestAddr))
	if bgNum < bftMgr.CandidateID-1 {
		return fmt.Errorf("invalid bgAdvice %d < %d", bgNum, bftMgr.CandidateID)
	}

	if bgAdvice.Height > bftMgr.CurHeight {
		ConsLog.Infof(LOGTABLE_CONS, "receive h %d > %d sync", bgAdvice.Height, bftMgr.CurHeight)
		bftMgr.MsgHandler.SyncEntrance(bgAdvice.Height, senderID)
		return nil
	}

	// 验证hash
	hash, err := ledger.GetBlockHash(bgAdvice.Height - 1)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS,
			"receive h %d > %d sync error: %v",
			bgAdvice.Height, bftMgr.CurHeight, err)
		return err
	}

	if !bytes.Equal(hash, bgAdvice.Hash) {
		ConsLog.Errorf(LOGTABLE_CONS,
			"invalid bgAdvice h %d hash %X != %X",
			bgAdvice.Height, bftMgr.CurHeight, err)
		return fmt.Errorf("invalid bgAdvice h %d hash %X != %X", bgAdvice.Height, bgAdvice.Hash, hash)
	}

	if bftMgr.GetBgDemand(bgNum) != nil {
		return fmt.Errorf("bgAdvice already +2/3")
	}

	bftMgr.PutBgAdvice(bgAdvice)

	adviceSlice := bftMgr.GetBgAdviceSlice(bgNum)

	if len(adviceSlice) > len(bftMgr.TotalValidators.Validators)/3 {
		ConsLog.Infof(LOGTABLE_CONS,
			"bgNum %d receive num > %d follow",
			bgNum, len(bftMgr.TotalValidators.Validators)/3)
		bgAdviceMsg, err := bftMgr.BuildBgAdvice(bftMgr.CurHeight, bgNum)
		if err != nil {
			return err
		}

		bgAdviceMsg.PeerID = senderID

		if bgNum > bftMgr.CandidateID {
			atomic.StoreUint64(&bftMgr.CandidateID, bgNum)
		}

		if !bftMgr.HasBgAdvice(bftMgr.MsgHandler.digestAddr, bgNum) {
			ConsLog.Infof(LOGTABLE_CONS, "broadcast bgNum %d", bgNum)
			bftMgr.BroadcastMsgToAllVP(bgAdviceMsg)

			select {
			case bftMgr.InternalMsgQueue <- bgAdviceMsg:
			default:
				ConsLog.Warningf(LOGTABLE_CONS, "recv msg chan full")
			}

		} else {
			ConsLog.Infof(LOGTABLE_CONS, "has broadcast bgAdvice bgNum %d", bgNum)
		}

	}

	if err := bftMgr.CheckResetBft(bgAdvice); err != nil {
		return err
	}

	return nil
}

func (bftMgr *BftMgr) CheckResetBft(bgAdvice *cmn.BftGroupSwitchAdvice) error {
	adviceSlice := bftMgr.GetBgAdviceSlice(bgAdvice.BftGroupNum)

	if len(adviceSlice) > 2*len(bftMgr.TotalValidators.Validators)/3 && bftMgr.CheckNewBgProposer(bgAdvice.BftGroupNum) {
		ConsLog.Infof(LOGTABLE_CONS,
			"bgAdvice +2/3 %v",
			PrintBgAdviceSlice(adviceSlice))
		bgDemandMsg, err := bftMgr.BuildBgDemand(bgAdvice.BftGroupNum, adviceSlice)
		if err != nil {
			return err
		}

		bftMgr.BroadcastMsgToAllVP(bgDemandMsg)
		ConsLog.Infof(LOGTABLE_CONS, "send bgDemand bgNum %d", bgAdvice.BftGroupNum)

		bgDemandMsg.PeerID = p2pNetwork.GetLocatePeerID()

		select {
		case bftMgr.InternalMsgQueue <- bgDemandMsg:
		default:
			ConsLog.Warningf(LOGTABLE_CONS, "recv msg chan full")
		}

	}

	return nil
}

func (bftMgr *BftMgr) CheckNewBgProposer(bgNum uint64) bool {
	newBgProposerAddr, err := bftMgr.GetNewBgProposerAddr(bgNum)
	if err != nil {
		return false
	}

	if bytes.Equal(bftMgr.MsgHandler.digestAddr, newBgProposerAddr) {
		return true
	}

	return false
}

func (bftMgr *BftMgr) GetNewBgProposerAddr(bgNum uint64) (util.HexBytes, error) {
	totalValsLen := len(bftMgr.TotalValidators.Validators)
	if totalValsLen == 0 {
		return nil, errors.New("total validators are empty")
	}

	proposerIndex := bgNum % uint64(totalValsLen)

	validators := make([]*types.Validator, len(bftMgr.TotalValidators.Validators))
	for i, val := range bftMgr.TotalValidators.Validators {
		validators[i] = val.Copy()
	}
	sort.Sort(types.ValidatorsByAddress(validators))

	ConsLog.Infof(LOGTABLE_CONS, "sorted validators %v", validators)

	return util.HexBytes(validators[proposerIndex].Address), nil
}

func (bftMgr *BftMgr) AddOwnBgCandidate(bgNum uint64) error {
	blkHash, err := ledger.GetBlockHash(bftMgr.CurHeight - 1)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "get blk hash err %v", err)
		return fmt.Errorf("get blk hash err %v", err)
	}

	advice := &cmn.BftGroupSwitchAdvice{
		Height:      bftMgr.CurHeight,
		Hash:        blkHash,
		BftGroupNum: bgNum,
		DigestAddr:  bftMgr.MsgHandler.digestAddr,
	}

	bftMgr.PutBgCandidate(advice)

	return nil
}

func (bftMgr *BftMgr) BuildBgDemand(bgNum uint64, bgAdviceSlice []*cmn.BftGroupSwitchAdvice) (*cmn.PeerMessage, error) {
	for _, advice := range bgAdviceSlice {
		bftMgr.PutBgCandidate(advice)
	}

	err := bftMgr.AddOwnBgCandidate(bgNum)
	if err != nil {
		return nil, err
	}

	bgCandidateSlice := bftMgr.GetBgCandidate(bgNum)

	ConsLog.Infof(LOGTABLE_CONS, "bgNum %d candidate %v", bgNum, PrintBgAdviceSlice(bgCandidateSlice))

	newBg, err := bftMgr.MakeNewBgFromBgCandidateSlice(bgNum, bgCandidateSlice)
	if err != nil {
		return nil, err
	}

	currentVRF, err := bftMgr.getCurrentVRF()
	if err != nil {
		return nil, err
	}

	height, err := ledger.GetBlockHeight()
	if err != nil {
		return nil, err
	}

	sk := msp.GetLocalPrivKey()

	VRFBgData := &vrf.VRFBgData{
		BgNum:    bgNum,
		BlockNum: height - 1,
		PrevVrf:  currentVRF,
	}

	ConsLog.Debugf(LOGTABLE_CONS, "(newBgEle) vrfBgData bgNum %d blockNum %d preVrf %x", VRFBgData.BgNum, VRFBgData.BlockNum, VRFBgData.PrevVrf)
	newVRFValue, newVRFProof, err := vrf.ComputeVRF4bg(sk, VRFBgData, msp.GetAlgType())
	if err != nil {
		return nil, err
	}

	demand := &cmn.BftGroupSwitchDemand{
		DigestAddr:         bftMgr.MsgHandler.digestAddr,
		BftGroupNum:        bgNum,
		BftGroupCandidates: bgCandidateSlice,
		NewBftGroup:        newBg,
		VRFValue:           newVRFValue,
		VRFProof:           newVRFProof,
	}

	bftMgr.PutBgDemand(demand)

	demandData, err := json.Marshal(demand)
	if err != nil {
		return nil, err
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: demandData,
		Type:    cmn.TDMType_MsgBgDemand,
	}

	tdmMsgSign, err := bftMgr.MsgHandler.Sign(tdmMsg)
	if err != nil {
		return nil, err
	}

	tdmData, err := json.Marshal(tdmMsgSign)

	if err != nil {
		return nil, err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return nil, err
	}

	peerMsg := &cmn.PeerMessage{
		Sender:      bftMgr.MsgHandler.ID,
		Msg:         conData,
		IsBroadCast: false,
	}

	return peerMsg, nil
}

func (bftMgr *BftMgr) MakeNewBgFromBgCandidateSlice(bgNum uint64, bgCandidateSlice []*cmn.BftGroupSwitchAdvice) ([]*cmn.BftGroupSwitchAdvice, error) {
	bgCandidateLen := len(bgCandidateSlice)
	doBftNum := bftMgr.BftNumber

	if bgCandidateLen == int(doBftNum) {
		ConsLog.Infof(LOGTABLE_CONS, "(newBgEle) bgNum %d bgCandidate len %d <= bftNum %d direct ret new bg %v",
			bgNum, bgCandidateLen, doBftNum, PrintBgAdviceSlice(bgCandidateSlice))

		return bgCandidateSlice, nil
	}

	candidateH := bgCandidateSlice[0].Height
	for i := 1; i < len(bgCandidateSlice); i++ {
		if candidateH != bgCandidateSlice[i].Height {
			return nil, fmt.Errorf("(newBgEle) bgNum %d candidateH %d != %d addr %v <> addr %v",
				bgNum, candidateH, bgCandidateSlice[i].Height, util.HexBytes(bgCandidateSlice[0].DigestAddr),
				util.HexBytes(bgCandidateSlice[i].DigestAddr))
		}
	}

	return bftMgr.bgSelectByVRF(bgCandidateSlice)
}

func (bftMgr *BftMgr) bgSelectByVRF(bgCandidateSlice []*cmn.BftGroupSwitchAdvice) ([]*cmn.BftGroupSwitchAdvice, error) {
	currentVRF, err := bftMgr.getCurrentVRF()
	if err != nil {
		return nil, err
	}

	newValSlice, err := vrf.CalcBFTGroupMembersByVRF(currentVRF, bgCandidateSlice, int(bftMgr.BftNumber))
	if err != nil {
		return nil, err
	}

	return newValSlice, nil
}

// 拿出当前的VRFValue
func (bftMgr *BftMgr) getCurrentVRF() ([]byte, error) {
	height, err := ledger.GetBlockHeight()
	if err != nil {
		return nil, err
	}

	block, err := ledger.GetBlock(height - 1)
	if err != nil {
		return nil, err
	}

	if block == nil {
		return nil, fmt.Errorf("blk %d is nil", height-1)
	}

	tdmBlk, err := types.Standard2Cons(block)
	if err != nil {
		return nil, err
	}

	if tdmBlk.Validators == nil || tdmBlk.Validators.BgVRFValue == nil ||
		len(tdmBlk.Validators.BgVRFValue) == 0 {
		valLastHeightChanged := tdmBlk.LastHeightChanged
		lastValidatorChangeHeightBlk, err := ledger.GetBlock(tdmBlk.LastHeightChanged)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "tdm get blk %d err %v", valLastHeightChanged, err)
			return nil, err
		}

		if lastValidatorChangeHeightBlk == nil {
			ConsLog.Errorf(LOGTABLE_CONS, "blk %d is nil", valLastHeightChanged)
			return nil, fmt.Errorf(" blk %d is nil", valLastHeightChanged)
		}

		lastTdmBlk, err := types.Standard2Cons(lastValidatorChangeHeightBlk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "tdm get blk %d err %v", valLastHeightChanged, err)
			return nil, err
		}

		if lastTdmBlk.Validators == nil || lastTdmBlk.Validators.BgVRFValue == nil ||
			len(lastTdmBlk.Validators.BgVRFValue) == 0 {
			ConsLog.Errorf(LOGTABLE_CONS, "tdm get BgVRFValue h %d err %v", valLastHeightChanged, err)
			return nil, fmt.Errorf("%d load validators err", valLastHeightChanged)
		}

		return lastTdmBlk.Validators.BgVRFValue, nil
	}

	return tdmBlk.Validators.BgVRFValue, nil
}

func (bftMgr *BftMgr) bgSelectBySort(bgNum uint64, bgCandidateSlice []*cmn.BftGroupSwitchAdvice) ([]*cmn.BftGroupSwitchAdvice, error) {
	addr, err := bftMgr.GetNewBgProposerAddr(bgNum)
	if err != nil {
		return nil, err
	}

	bgcOrder := &BgCandidateInOrder{
		candidates:    bgCandidateSlice,
		bgNum:         bgNum,
		curBg:         bftMgr.CurBftGroup,
		newBgProposer: addr,
	}

	bgCandidateLen := len(bgCandidateSlice)
	doBftNum := bftMgr.BftNumber

	sort.Sort(bgcOrder)
	ConsLog.Infof(LOGTABLE_CONS,
		"(newBgEle) bgNum %d bgCandidate len %d > bftNum %d sort then ret new bg %v",
		bgNum, bgCandidateLen, doBftNum,
		PrintBgAdviceSlice(bgcOrder.candidates[:doBftNum]))

	return bgcOrder.candidates[:doBftNum], nil
}

func (bftMgr *BftMgr) BuildBgAdvice(height uint64, bgNum uint64) (*cmn.PeerMessage, error) {
	blkHash, err := ledger.GetBlockHash(height - 1)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "get blk hash err %v", err)
		return nil, err
	}

	advice := &cmn.BftGroupSwitchAdvice{
		Height:      height,
		Hash:        blkHash,
		BftGroupNum: bgNum,
		DigestAddr:  bftMgr.MsgHandler.digestAddr,
	}

	adviceData, err := json.Marshal(advice)
	if err != nil {
		return nil, err
	}

	tdmMsg := &cmn.TDMMessage{
		Payload: adviceData,
		Type:    cmn.TDMType_MsgBgAdvice,
	}

	tdmMsgSign, err := bftMgr.MsgHandler.Sign(tdmMsg)
	if err != nil {
		return nil, err
	}
	tdmData, err := json.Marshal(tdmMsgSign)
	if err != nil {
		return nil, err
	}
	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.Tendermint),
		Payload: tdmData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return nil, err
	}
	peerMsg := &cmn.PeerMessage{
		Sender:      bftMgr.MsgHandler.ID,
		Msg:         conData,
		IsBroadCast: false,
	}

	return peerMsg, nil
}

type BgCandidateInOrder struct {
	candidates    []*cmn.BftGroupSwitchAdvice
	newBgProposer util.HexBytes
	curBg         BftGroup
	bgNum         uint64
}

func (bgc BgCandidateInOrder) Len() int {
	return len(bgc.candidates)
}

// 进入新的共识组规则
// 1. 发起人先选入新的共识组
// 2. 高度优先
// 3. 不在上一个共识组的优先
// 4. hash(addr, bgNumber) bytes 大的优先
func (bgc BgCandidateInOrder) Less(i, j int) bool {
	if bytes.Equal(bgc.candidates[i].DigestAddr, bgc.newBgProposer) {
		return true
	} else if bytes.Equal(bgc.candidates[j].DigestAddr, bgc.newBgProposer) {
		return false
	} else if bgc.candidates[i].Height > bgc.candidates[j].Height {
		return true
	} else if bgc.candidates[i].Height < bgc.candidates[j].Height {
		return false
	} else if !bgc.curBg.Exist(util.HexBytes(bgc.candidates[i].DigestAddr)) &&
		bgc.curBg.Exist(util.HexBytes(bgc.candidates[j].DigestAddr)) {
		return true
	} else if bgc.curBg.Exist(util.HexBytes(bgc.candidates[i].DigestAddr)) &&
		!bgc.curBg.Exist(util.HexBytes(bgc.candidates[j].DigestAddr)) {
		return false
	} else {
		hashed_i, err := bgc.HashBgNumAndAddr(bgc.bgNum, util.HexBytes(bgc.candidates[i].DigestAddr))
		if err != nil {
			return true
		}

		hashed_j, err := bgc.HashBgNumAndAddr(bgc.bgNum, util.HexBytes(bgc.candidates[j].DigestAddr))
		if err != nil {
			return true
		}

		result := bytes.Compare(hashed_i, hashed_j)
		if result > 0 {
			return false
		} else if result < 0 {
			return true
		} else {
			ConsLog.Errorf(LOGTABLE_CONS, "hash conflict bgNum %v addr1 %v, addr2 %v",
				bgc.bgNum, util.HexBytes(bgc.candidates[i].DigestAddr), util.HexBytes(bgc.candidates[j].DigestAddr))
			return true
		}

	}
}

func (bgc BgCandidateInOrder) Swap(i, j int) {
	tmp := bgc.candidates[i]
	bgc.candidates[i] = bgc.candidates[j]
	bgc.candidates[j] = tmp
}

func (bgc BgCandidateInOrder) HashBgNumAndAddr(bgNum uint64, addr util.HexBytes) ([]byte, error) {
	args := []interface{}{bgNum, addr}
	bytes, err := json.Marshal(args)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "marshal bgNum addr err %v", err)
		return nil, err
	}
	//TODO Hash
	digest, err := msp.Hash256(bytes)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_CONS, "bccsp hash bgNum addr err %v", err)
		return nil, err
	}

	return digest, nil
}

func PrintBgAdviceSlice(bgAdviceSlice []*cmn.BftGroupSwitchAdvice) string {
	if bgAdviceSlice == nil {
		return "nil-bgAdviceSlice"
	}

	bgAdviceStr := make([]string, len(bgAdviceSlice))
	for i, bgAdvice := range bgAdviceSlice {
		bgAdviceStr[i] = PrintBgAdvice(bgAdvice)
	}

	return strings.Join(bgAdviceStr, "\n")
}

func PrintBgAdvice(bgAdvice *cmn.BftGroupSwitchAdvice) string {
	if bgAdvice == nil {
		return "nil-bgAdvice"
	}

	return fmt.Sprintf("{BlockNum %d Hash %v bgNum %d addr %v}",
		bgAdvice.Height, util.HexBytes(bgAdvice.Hash), bgAdvice.BftGroupNum, util.HexBytes(bgAdvice.DigestAddr))
}
