package dbft

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	dposStruct "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/dbft/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/db"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/ledger"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
	"github.com/json-iterator/go"
	"sort"
	"strconv"
)

func (dm *DBFTManager) HandleEpochChange(dposMsg *dposStruct.DPoSMessage, senderID uint64) error {
	epochChange := &dposStruct.EpochChange{}
	err := jsoniter.Unmarshal(dposMsg.Payload, epochChange)
	if err != nil {
		return err
	}

	epochNum := epochChange.EpochNo
	ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) receive epochNum %d epochChange <- %v", epochNum, util.HexBytes(epochChange.DigestAddr))

	if epochNum < dm.candidateNo.GetCurrentNo()-1 {
		return fmt.Errorf("invalid epochChange epochNum %d < %d", epochNum, dm.candidateNo.GetCurrentNo())
	}

	if epochChange.Height > dm.CurHeight {
		ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) receive h %d > %d sync", epochChange.Height, dm.CurHeight)
		dm.bftHandler.SyncEntrance(epochChange.Height, senderID)
		return nil
	}

	hash, err := ledger.GetBlockHash(epochChange.Height - 1)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "(epochChange) receive h %d < %d sync error: %v", epochChange.Height, dm.CurHeight, err)
		return err
	}

	if !bytes.Equal(hash, epochChange.Hash) {
		ConsLog.Errorf(LOGTABLE_DBFT, "invalid epochChange h %d hash %X != %X", epochChange.Height, epochChange.Hash, hash)
		return fmt.Errorf("invalid epochChange h %d hash %X != %X", epochChange.Height, epochChange.Hash, hash)
	}

	if dm.NewEpoch.IsExist(epochNum) {
		return fmt.Errorf("epochChange already +2/3")
	}

	dm.EpochChange.SetEpochChange(epochChange)
	if dm.EpochChange.IsMoreThanOneThridToCandidates(epochChange.EpochNo, len(dm.TotalValidators.Validators)) {
		ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) epochNum %d receive num > %d follow", epochNum, len(dm.TotalValidators.Validators)/3)
		epochChangeMsg, err := dm.BuildEpochChangeMsg(dm.CurHeight, epochNum)
		if err != nil {
			return err
		}

		if epochNum > dm.candidateNo.GetCurrentNo() {
			dm.candidateNo.SetNo(epochNum)
		}

		ok, err := dm.EpochChange.IsExist(epochChange)
		if err != nil {
			return err
		}
		if !ok {
			ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) broadcast epochNum %d ", epochNum)

			dm.BroadcastMsgToAllVP(epochChangeMsg)

			select {
			case dm.InternalMsgQueue <- epochChangeMsg:
			default:
				ConsLog.Warningf(LOGTABLE_DBFT, "tdm recvMsgChan full")
			}

		} else {
			ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) has broadcast epochChange epochNum %d ", epochNum)
		}

	}
	err = dm.CheckEnterNewEpoch(epochChange)
	if err != nil {
		ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) CheckEnterNewEpoch err %s", err.Error())
		return err
	}

	return nil
}

func (dm *DBFTManager) CheckEnterNewEpoch(epochChange *dposStruct.EpochChange) error {
	if dm.EpochChange.IsMajorityToCandidates(epochChange.EpochNo, len(dm.TotalValidators.Validators)) {
		if dm.CheckNewEpochSender(epochChange.EpochNo) {
			newEpochMsg, err := dm.BuildNewEpochMsg(epochChange.EpochNo)
			if err != nil {
				return err
			}

			dm.BroadcastMsgToAllVP(newEpochMsg)
			ConsLog.Infof(LOGTABLE_DBFT, "(newEpoch) send newEpoch bgNum %d", epochChange.EpochNo)

			select {
			case dm.InternalMsgQueue <- newEpochMsg:
			default:
				ConsLog.Warningf(LOGTABLE_DBFT, "dpos recvMsgChan full")
			}
		}
	}
	return nil
}

func (dm *DBFTManager) CheckNewEpochSender(bgNum uint64) bool {
	newEpochSenderAddr, err := dm.GetNewEpochSenderAddr(bgNum)
	if err != nil {
		return false
	}

	ConsLog.Debugf(LOGTABLE_DBFT, "epochNo %d newEpochSender addr %v, my addr %v", bgNum, newEpochSenderAddr, dm.bftHandler.GetDigestAddr())

	if bytes.Equal(dm.bftHandler.GetDigestAddr(), newEpochSenderAddr) {
		return true
	}

	return false
}

func (dm *DBFTManager) GetNewEpochSenderAddr(epochNo uint64) (util.HexBytes, error) {
	totalValsLen := len(dm.TotalValidators.Validators)
	if totalValsLen == 0 {
		return nil, errors.New("total validators are empty")
	}

	proposerIndex := epochNo % uint64(totalValsLen)

	validators := make([]*types.Validator, len(dm.TotalValidators.Validators))
	for i, val := range dm.TotalValidators.Validators {
		validators[i] = val.Copy()
	}
	sort.Sort(types.ValidatorsByAddress(validators))

	ConsLog.Infof(LOGTABLE_DBFT, "(dm) sorted validators %v", validators)

	return util.HexBytes(validators[proposerIndex].Address), nil
}

func (dm *DBFTManager) HandleNewEpoch(dposMsg *dposStruct.DPoSMessage) (*dposStruct.NewEpoch, error) {
	newEpoch := new(dposStruct.NewEpoch)
	err := jsoniter.Unmarshal(dposMsg.Payload, newEpoch)
	if err != nil {
		return nil, err
	}
	ConsLog.Infof(LOGTABLE_DBFT, "(epochChange) receive epochNo %d NewEpoch <- %v", newEpoch.EpochNo, util.HexBytes(newEpoch.DigestAddr))

	newEpochNo := newEpoch.EpochNo
	if newEpochNo < dm.candidateNo.GetCurrentNo() {
		return nil, fmt.Errorf("(newEpoch) net epoch %d !=  my %d",
			newEpochNo, dm.candidateNo.GetCurrentNo())
	}

	newEpochSenderAddr, err := dm.GetNewEpochSenderAddr(newEpochNo)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(newEpoch.DigestAddr, newEpochSenderAddr) {
		return nil, fmt.Errorf("(newEpoch) send not Expect, send %s Expect %s",
			hex.EncodeToString(newEpoch.DigestAddr), hex.EncodeToString(newEpochSenderAddr))
	}
	if !dm.EpochChange.IsMoreThanOneThridToCandidates(newEpochNo, len(dm.TotalValidators.Validators)) {
		return nil, fmt.Errorf("(newEpoch) recv epoch %d epochChange not enough", newEpochNo)
	}
	selfEpoch, err := dm.BuildNewEpoch(newEpochNo)
	if err != nil {
		return nil, err
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	witnessesList := []*types.Validator{}
	err = json.Unmarshal(newEpoch.Witnesss, &witnessesList)
	if err != nil {
		return nil, err
	}
	netEpoch := &Epoch{
		EpochNo:       newEpochNo,
		DependEpochNo: newEpoch.DependEpochNo,
		WitnessList:   witnessesList,
		BeginNum:      newEpoch.Begin,
		EndNum:        newEpoch.End,
	}
	selfEpoch.ReSetBeginBlk(dm.bftHandler.Height)
	if !dm.checkNewEpochMatch(netEpoch, selfEpoch) {
		return nil, fmt.Errorf("newEpochNo %d  not match with self", newEpochNo)
	}
	err = dm.ResetEpoch(netEpoch)
	if err != nil {
		return nil, fmt.Errorf("newEpochNo %d ResetEpoch err %s", netEpoch.EpochNo, err.Error())
	}
	return newEpoch, nil
}
func (dm *DBFTManager) checkNewEpochMatch(net, self *Epoch) bool {
	if net.BeginNum != self.BeginNum {
		ConsLog.Warningf(LOGTABLE_DBFT, "(newEpoch) the BeginNum net %d self %d",
			net.BeginNum, self.BeginNum)
		return false
	}
	if net.EndNum != self.EndNum {
		ConsLog.Warningf(LOGTABLE_DBFT, "(newEpoch) the EndNum net %d self %d",
			net.EndNum, self.EndNum)
		return false
	}
	if net.DependEpochNo != self.DependEpochNo {
		ConsLog.Warningf(LOGTABLE_DBFT, "(newEpoch) the dependEpochNo net %d self %d",
			net.DependEpochNo, self.DependEpochNo)
		return false
	}
	if len(net.WitnessList) != len(self.WitnessList) {
		ConsLog.Warningf(LOGTABLE_DBFT, "(newEpoch) the witnesses len net %d self %d",
			len(net.WitnessList), len(self.WitnessList))
		return false
	}

	var match bool
	for _, netWitness := range net.WitnessList {
		match = false
		for _, selfWitness := range self.WitnessList {
			if bytes.Equal(netWitness.Hash(), selfWitness.Hash()) {
				match = true
				break
			}
		}
		if !match {
			ConsLog.Warningf(LOGTABLE_DBFT, "(newEpoch) the witness not match net %s self %s",
				net.WitnessList, self.WitnessList)
			return false
		}
	}
	return true
}

func GetKey(chainId string, epochNo uint64) string {
	return chainId + strconv.FormatUint(epochNo, 10)
}

func (dm *DBFTManager) RecordEpochInfo(epochInfo *Epoch) error {

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	epochData, err := json.Marshal(epochInfo)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "RecordEpochInfo Marshal err %s", err.Error())
		return err
	}
	key := GetKey(dm.ChainID, epochInfo.EpochNo)
	err = db.KVDB.Put([]byte(key), epochData)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "RecordEpochInfo Put err %s", err.Error())
		return err
	}
	return nil
}

func LoadEpochInfo(chainid string, epochNo uint64) (*Epoch, error) {
	key := GetKey(chainid, epochNo)
	epochData, err := db.KVDB.Get([]byte(key))
	var epoch *Epoch
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(epochData, &epoch)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "LoadEpochInfo Unmarshal err %s", err.Error())
		return nil, err
	}
	return epoch, nil

}
