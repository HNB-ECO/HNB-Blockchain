package dbft

import (
	"HNB/consensus/algorand/types"
	"HNB/consensus/consensusManager/comm/consensusType"
	dbftComm "HNB/consensus/dbft/common"
	dposMsg "HNB/consensus/dbft/common"
	"HNB/ledger"
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/json-iterator/go"

	cmn "HNB/consensus/algorand/common"
)

type EpochChangeMap struct {
	sync.RWMutex
	ecList map[uint64][]*dposMsg.EpochChange
}

func NewEpochChangeMap() *EpochChangeMap {
	return &EpochChangeMap{
		ecList: make(map[uint64][]*dposMsg.EpochChange),
	}
}

func (ecm *EpochChangeMap) GetEpochChangeLen(epochNo uint64) int {
	ecm.RLock()
	defer ecm.RUnlock()
	return len(ecm.ecList[epochNo])
}
func (ecm *EpochChangeMap) IsExist(epoch *dposMsg.EpochChange) (bool, error) {

	ecm.RLock()
	defer ecm.RUnlock()
	ecList := ecm.ecList[epoch.EpochNo]
	for _, e := range ecList {
		if bytes.Equal(e.DigestAddr, epoch.DigestAddr) {
			return true, nil
		}
	}
	return false, nil
}

func (ecm *EpochChangeMap) SetEpochChange(epoch *dposMsg.EpochChange) error {
	if epoch == nil {
		return fmt.Errorf("epoch is nil")
	}
	ecm.Lock()
	ecm.ecList[epoch.EpochNo] = append(ecm.ecList[epoch.EpochNo], epoch)
	ecm.Unlock()
	return nil
}

func (ecm *EpochChangeMap) GetEpochChange(epochNo uint64) []*dposMsg.EpochChange {
	ecm.RLock()
	defer ecm.RUnlock()
	return ecm.ecList[epochNo]
}

func (ecm *EpochChangeMap) IsMajorityToCandidates(epochNo uint64, candidatesNum int) bool {
	ecm.RLock()
	defer ecm.RUnlock()
	if len(ecm.ecList[epochNo]) > candidatesNum*2/3 {
		return true
	}
	return false
}

func (ecm *EpochChangeMap) IsMoreThanOneThridToCandidates(epochNo uint64, candidatesNum int) bool {
	ecm.RLock()
	defer ecm.RUnlock()
	if len(ecm.ecList[epochNo]) > candidatesNum/3 {
		return true
	}
	return false
}

func (ecm *EpochChangeMap) DelEpochChange(epochNo uint64) {
	ecm.RLock()
	defer ecm.RUnlock()
	delete(ecm.ecList, epochNo)
}

type NewEpochMap struct {
	sync.RWMutex
	ne map[uint64]*dposMsg.NewEpoch
}

func NewNewEpochMap() *NewEpochMap {
	return &NewEpochMap{
		ne: make(map[uint64]*dposMsg.NewEpoch),
	}
}

func (ne *NewEpochMap) IsExist(epochNo uint64) bool {
	ne.RLock()
	defer ne.RUnlock()
	if ne.ne[epochNo] == nil {
		return false
	}
	return true
}
func (ne *NewEpochMap) SetNewEpochMap(newEpochMap *dposMsg.NewEpoch) error {
	if newEpochMap == nil {
		return fmt.Errorf("newEpochMap is nil")
	}
	ne.Lock()
	ne.ne[newEpochMap.EpochNo] = newEpochMap
	ne.Unlock()
	return nil
}

func (ne *NewEpochMap) GetNewEpochMap(epochNo uint64) *dposMsg.NewEpoch {
	ne.RLock()
	defer ne.RUnlock()
	return ne.ne[epochNo]
}

func (ne *NewEpochMap) DelNewEpochMap(epochNo uint64) {
	ne.Lock()
	delete(ne.ne, epochNo)
	ne.Unlock()
}

func (dm *DBFTManager) BuildEpochChangeMsg(height, epochNo uint64) (*cmn.PeerMessage, error) {

	ledger.GetBlockHash(height - 1)
	blkHash, err := ledger.GetBlockHash(height - 1)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "get blk hash err %v", err)
	}

	epochChange := &dposMsg.EpochChange{
		EpochNo: epochNo,
		Height:  height,
		Hash:    blkHash,
		//IsTimeout:	 isTimeout,
		DigestAddr: dm.bftHandler.GetDigestAddr(),
	}

	epochChangeData, err := jsoniter.Marshal(epochChange)
	if err != nil {
		return nil, err
	}

	dposMsg := &dposMsg.DPoSMessage{
		Payload: epochChangeData,
		ChainId: dm.ChainID,
		Type:    dposMsg.DPoSType_MsgEpochChange,
	}

	dposMsgSign, err := dm.Sign(dposMsg)
	if err != nil {
		return nil, err
	}

	dposData, err := jsoniter.Marshal(dposMsgSign)

	if err != nil {
		return nil, err
	}

	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.DBFT),
		Payload: dposData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return nil, err
	}

	peerMsg := &cmn.PeerMessage{
		Sender:      dm.bftHandler.ID,
		Msg:         conData,
		IsBroadCast: false,
	}

	return peerMsg, nil
}

func (dm *DBFTManager) EpochChangeReq(height uint64) {

}

func (dm *DBFTManager) BuildNewEpochMsg(newEpochNo uint64) (*cmn.PeerMessage, error) {
	epoch, err := dm.BuildNewEpoch(newEpochNo)
	if err != nil {
		return nil, err
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	witnessesData, err := json.Marshal(epoch.WitnessList)
	if err != nil {
		return nil, err
	}
	newEpoch := &dposMsg.NewEpoch{
		EpochNo:       newEpochNo,
		DependEpochNo: epoch.DependEpochNo,
		Witnesss:      witnessesData,
		DigestAddr:    dm.bftHandler.GetDigestAddr(),
		Begin:         dm.bftHandler.Height,
		End:           dm.bftHandler.Height,
	}
	newEpochData, err := jsoniter.Marshal(newEpoch)
	if err != nil {
		return nil, err
	}

	dposMsg := &dposMsg.DPoSMessage{
		Payload: newEpochData,
		ChainId: dm.ChainID,
		Type:    dposMsg.DPoSType_MsgNewEpoch,
	}

	dposMsgSign, err := dm.Sign(dposMsg)
	if err != nil {
		return nil, err
	}

	dposData, err := jsoniter.Marshal(dposMsgSign)

	if err != nil {
		return nil, err
	}

	conMsg := &consensusType.ConsensusMsg{
		Type:    int(consensusType.DBFT),
		Payload: dposData,
	}

	conData, err := json.Marshal(conMsg)
	if err != nil {
		return nil, err
	}

	peerMsg := &cmn.PeerMessage{
		Sender:      dm.bftHandler.ID,
		Msg:         conData,
		IsBroadCast: false,
	}

	return peerMsg, nil
}

func (dm *DBFTManager) BuildNewEpoch(epochNo uint64) (*Epoch, error) {
	ConsLog.Infof(LOGTABLE_DBFT, "BuildNewEpoch in")
	witnesses, dependEpochNo, err := dm.GetWitnesses()
	ConsLog.Infof(LOGTABLE_DBFT, "BuildNewEpoch GetWitnesses")
	if err != nil {
		return nil, err
	}
	return &Epoch{
		DependEpochNo: dependEpochNo,
		WitnessList:   witnesses,
	}, nil
}

func (dm *DBFTManager) GetWitnesses() ([]*types.Validator, uint64, error) {
	return dm.beginWitnessSelection(dm.epoch, dm.TotalValidators, dm.epochList)
}

func (dm *DBFTManager) SendNewEpoch() {

}

func (dm *DBFTManager) IsFrozenTokenToTaget() bool {
	allToken := dbftComm.GetAllToken()
	frozenToken := dbftComm.GetEpocFrozenToken(dm.epoch.EpochNo)
	if frozenToken/allToken > 1/2 {
		return true
	}
	return false
}

func (dm *DBFTManager) GetPreviousEpochWitnesses() (*Epoch, error) {
	var newDependEpochNo uint64
	for i := dm.epoch.EpochNo - 1; i >= 0; i-- {
		if dm.epochList.isExist(i) {
			newDependEpochNo = i
			break
		}
	}
	epoch := dm.epochList.GetEpoch(newDependEpochNo)
	if epoch == nil {
		return nil, fmt.Errorf("get epoch [%s] err", newDependEpochNo)
	}
	return epoch, nil
}

type NewEpochNotify struct {
	sync.RWMutex
	notify map[uint64]chan uint64
}

func NewNewEpochNotify() *NewEpochNotify {
	return &NewEpochNotify{
		notify: make(map[uint64]chan uint64),
	}
}

func (nen *NewEpochNotify) SetNewEpochNotify(epochNo uint64) {
	nen.RLock()
	nen.notify[epochNo] = make(chan uint64)
	nen.RUnlock()
}

func (nen *NewEpochNotify) GetNewEpochNotify(epochNo uint64) (chan uint64, bool) {
	nen.RLock()
	defer nen.RUnlock()
	notify, ok := nen.notify[epochNo]
	return notify, ok
}

func (nen *NewEpochNotify) DelNewEpochNotify(epochNo uint64) {
	nen.RLock()
	delete(nen.notify, epochNo)
	nen.RUnlock()
}

type EpochChangeCache struct {
	sync.RWMutex
	cache map[uint64]struct{}
}

func NewEpochChangeCache() *EpochChangeCache {
	return &EpochChangeCache{
		cache: make(map[uint64]struct{}),
	}
}

func (ecc *EpochChangeCache) SetCache(height uint64) {
	ecc.Lock()
	ecc.cache[height] = struct{}{}
	ecc.Unlock()
}
func (ecc *EpochChangeCache) IsExist(height uint64) bool {
	ecc.RLock()
	defer ecc.RUnlock()
	_, ok := ecc.cache[height]
	return ok
}

func (ecc *EpochChangeCache) DelCache(height uint64) {
	ecc.Lock()
	delete(ecc.cache, height)
	ecc.Unlock()
}
