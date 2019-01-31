package dbft

import (
	"HNB/consensus/algorand/state"
	"HNB/consensus/algorand/types"
	"HNB/db"
	"HNB/msp"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"HNB/consensus/algorand"

	"HNB/consensus/algorand/msgHandler"

	appComm "HNB/appMgr/common"
	"HNB/common"
	"HNB/config"
	"HNB/ledger"
	bsComm "HNB/ledger/blockStore/common"
	"HNB/logging"
	"github.com/json-iterator/go"
)

var TARGET_BLK_COUNT int

var ConsLog logging.LogModule

const (
	LOGTABLE_DBFT string = "dbft"
)

var DS *DBFTServer

type DBFTCore struct {
	DPosManager   *DBFTManager
	PosVoteTable  string
	PosAssetTable string
	PosEpochTable string
}

type DBFTServer struct {
	*DBFTCore
}

func (s *DBFTServer) Start() {
	err := s.DBFTCore.NewConsensusCore()
	if err != nil {
		panic(err.Error())
	}
}

func (s *DBFTServer) Stop() {

}

func NewDBFTServer() *DBFTServer {
	dbft := NewDBFTCore()

	DS := &DBFTServer{
		dbft,
	}
	return DS
}

func NewDBFTCore() *DBFTCore {

	ConsLog = logging.GetLogIns()
	dbft := &DBFTCore{}
	return dbft
}

func (dbftCore *DBFTCore) NewConsensusCore() error {

	dbftCore.PosAssetTable = appComm.HNB + "_" + TABLE_POS_ASSET
	dbftCore.PosVoteTable = appComm.HNB + "_" + TABLE_POS_VOTE
	dbftCore.PosEpochTable = appComm.HNB + "_" + TABLE_EPOCH_INFO

	TARGET_BLK_COUNT = config.Config.EpochPeriod
	lastCommitState, err := dbftCore.Init()

	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "dbft init err %v", err)
		return err
	}

	handler, err := msgHandler.NewTDMMsgHandler(*lastCommitState)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "dbft newConsensusCore err %v", err)
		return err
	}

	dbftMgr, err := NewDBFTManager(*lastCommitState)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "dbft new dbftMgr err %v", err)
		return err
	}

	handler.RegisterIsProposerFunc(dbftMgr.isProposerByDPoS)
	handler.RegisterGeneProposalBlkFunc(geneProposalBlkByDPoS)
	handler.RegisterValidateProposalBlkFunc(validateProposalBlkByDPoS)
	//rhandler.RegisterConsSucceedFunc("frozenTokenAndRecord", dbftMgr.frozenTokenAndRecord)

	handler.RegisterStatusUpdatedFunc("checkEpoch", dbftMgr.CheckEpochChange)
	handler.RegisterStatusUpdatedFunc("coinbase", dbftMgr.CoinBase)


	dbftMgr.bftHandler = handler
	dbftCore.DPosManager = dbftMgr

	ConsLog.Infof(LOGTABLE_DBFT, "LastBlockHeight %d handler height %d", lastCommitState.LastBlockNum, handler.Height)

	dbftMgr.Start()

	ConsLog.Infof(LOGTABLE_DBFT, "*** Start DBFT SUCCESS ***")
	return nil
}

func (dbftCore *DBFTCore) GetCurrentEpochNo() uint64 {
	return dbftCore.DPosManager.epoch.EpochNo
}

func (dbftCore *DBFTCore) GetVotedCache() map[int64]bool {
	return dbftCore.DPosManager.votedCache
}

func (dbftCore *DBFTCore) Init() (*state.State, error) {
	curH, err := ledger.GetBlockHeight()
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "(dbftInit) dbft get height err %v", err.Error())
		return nil, err
	}

	ConsLog.Infof(LOGTABLE_DBFT, "(dbftInit) curH %d", curH)
	if curH == 0 {
		genesisBlk, err := dbftCore.MakeGenesisBlk()
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "(dbftInit) MakeGenesisBlk err %v", err)
			return nil, err
		}

		uniformBlk, err := types.ConsToStandard(genesisBlk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "(dbftInit) CustomTDMBlk2UniformBlk err %v", err)
			return nil, err
		}

		err = ledger.WriteLedger(uniformBlk, nil, nil)
		if err != nil {
			return nil, err
		}

		lastCommitState, err := dbftCore.MakeGenesisLastCommitState(uniformBlk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "(dbftInit) dbft make genesis lastCommitState err %v", err)
			return nil, err
		}

		epochInfo := &Epoch{}
		var json = jsoniter.ConfigCompatibleWithStandardLibrary
		err = json.Unmarshal(lastCommitState.Validators.Extend, &epochInfo)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "Marshal err %s", err.Error())
			return nil, err
		}
		epochInfo.EpochNo = 0
		epochInfo.BeginNum = 0
		epochInfo.EndNum = 1
		dbftCore.GeneRecordEpochInfo(epochInfo)

		return lastCommitState, nil

	} else {
		blk, err := ledger.GetBlock(curH - 1)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "(dbftInit) dbft get blk %d err %v", curH-1, err)
			return nil, err
		}

		if blk == nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "(dbftInit) blk %d is nil", curH-1)
			return nil, fmt.Errorf("(dbftInit) blk %d is nil", curH-1)
		}

		lastCommitState, err := dbftCore.LoadLastCommitStateFromBlk(blk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "(dbftInit) dbft load lastCommitState from blk %d err %v", curH-1, err)
			return nil, err
		}

		return lastCommitState, nil
	}
}

func (dbftCore *DBFTCore) GeneRecordEpochInfo(epochInfo *Epoch) error {

	epochInfo.TargetBlkNum = TARGET_BLK_COUNT
	epochInfoData, err := json.Marshal(epochInfo)
	if err != nil {
		return fmt.Errorf("(newEpoch) GeneRecordEpochInfo marshal err %s", err.Error())
	}
	key := GetKey(appComm.HNB, epochInfo.EpochNo)
	err = db.KVDB.Put([]byte(key), epochInfoData)
	if err != nil {
		return fmt.Errorf("(newEpoch) GeneRecordEpochInfo put err %s", err.Error())
	}
	return nil
}

func (dbftCore *DBFTCore) LoadLastCommitStateFromBlk(block *bsComm.Block) (*state.State, error) {
	tdmBlk, err := types.Standard2Cons(block)
	if err != nil {
		return nil, err
	}

	curValidators, err := dbftCore.LoadValidators(block)
	if err != nil {
		return nil, err
	}

	var lastValidators *types.ValidatorSet
	if block.Header.BlockNum == 0 {
		lastValidators = types.NewValidatorSet(nil, 0, nil, nil, nil)
	} else {
		prevBlk, err := ledger.GetBlock(block.Header.BlockNum - 1)
		if prevBlk == nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "tdm get last blk %d nil %v", block.Header.BlockNum-1, err)
			return nil, fmt.Errorf("tdm get last blk %d nil %v", block.Header.BlockNum-1, err)
		}

		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "tdm get last blk %d err %v", block.Header.BlockNum-1, err)
			return nil, err
		}

		lastValidators, err = dbftCore.LoadValidators(prevBlk)
		if err != nil {
			return nil, err
		}
	}

	var lastBlkID types.BlockID
	if tdmBlk.BlockNum != 0 {
		lastBlkID = tdmBlk.CurrentCommit.BlockID
	}

	lblkPreviousHash, err := ledger.CalcBlockHash(block)
	if err != nil {
		return nil, err
	}
	return &state.State{
		LastBlockNum:                tdmBlk.BlockNum,
		LastBlockTime:               tdmBlk.Time,
		LastBlockID:                 lastBlkID,
		LastBlockTotalTx:            tdmBlk.Header.TotalTxs,
		Validators:                  curValidators,
		LastValidators:              lastValidators,
		LastHeightValidatorsChanged: tdmBlk.LastHeightChanged,
		PreviousHash:                lblkPreviousHash,
	}, nil
}

func (dbftCore *DBFTCore) LoadValidators(blk *bsComm.Block) (valSet *types.ValidatorSet, err error) {
	tdmBlk, err := types.Standard2Cons(blk)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "dbft get blk %d err %v", tdmBlk.LastHeightChanged, err)
		return nil, err
	}

	valSet = tdmBlk.Validators

	if valSet == nil {
		lastValidatorChangeHeightBlk, err := ledger.GetBlock(tdmBlk.LastHeightChanged)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "tdm get blk %d err %v", tdmBlk.LastHeightChanged, err)
			return nil, err
		}

		if lastValidatorChangeHeightBlk == nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "blk %d is nil", tdmBlk.LastHeightChanged)
			return nil, fmt.Errorf(" blk %d is nil", tdmBlk.LastHeightChanged)
		}

		lastTdmBlk, err := types.Standard2Cons(lastValidatorChangeHeightBlk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_DBFT, "tdm get blk %d err %v", tdmBlk.LastHeightChanged, err)
			return nil, err
		}

		if lastTdmBlk.Validators == nil {
			return nil, fmt.Errorf("%d load validators err", tdmBlk.LastHeightChanged)
		}

		valSet = lastTdmBlk.Validators
	}

	return valSet, nil
}

func (dbftCore *DBFTCore) MakeGenesisLastCommitState(blk *bsComm.Block) (*state.State, error) {
	validatorSet, err := dbftCore.LoadGeneValidators()
	if err != nil {
		return nil, err
	}

	preblkHash, err := ledger.CalcBlockHash(blk)
	if err != nil {
		return nil, err
	}
	config := config.Config

	return &state.State{
		LastBlockNum:                     0,
		LastBlockID:                      types.BlockID{},
		LastBlockTime:                    config.GenesisTime,
		Validators:                       validatorSet,
		LastValidators:                   types.NewValidatorSet(nil, 0, nil, nil, nil),
		LastHeightValidatorsChanged:      0,
		LastHeightConsensusParamsChanged: 0,
		PreviousHash:                     preblkHash,
	}, nil
}

func (dbftCore *DBFTCore) MakeGenesisBlk() (*types.Block, error) {
	validatorSet, err := dbftCore.LoadGeneValidators()
	if err != nil {
		return nil, err
	}

	configMap := make(map[string]string)
	configMap[algorand.GENE_BFTGROUP] = validatorSet.PeerIDStr()
	configMap[algorand.GENE_BFTGROUP_NUM] = strconv.Itoa(0)

	tx := common.Transaction{
		Payload:      []byte("genesis"),
		ContractName: appComm.HNB,
	}

	signer := msp.GetSigner()
	tx.Txid = signer.Hash(&tx)

	var txs []*common.Transaction
	txs = append(txs, &tx)
	geneTDMTxs, err := types.Tx2TDMTx(txs)
	if err != nil {
		return nil, err
	}

	genesisiTime, _ := time.Parse("2006-01-02 15:04:05", "2018-10-19 00:00:00")
	genesisHeader := &types.Header{
		BlockNum: 0,
		Time:     genesisiTime,
		NumTxs:   int64(len(geneTDMTxs)),
	}

	genesisBlk := &types.Block{
		Header: genesisHeader,
		Data: &types.Data{
			Txs: geneTDMTxs,
		},
		ValidatorInfo: &types.ValidatorInfo{
			LastHeightChanged: 0,
			Validators:        validatorSet,
		},
		LastCommit:    &types.Commit{},
		CurrentCommit: &types.Commit{},
		Evidence:      types.EvidenceData{},
	}

	return genesisBlk, nil
}

func (dbftCore *DBFTCore) LoadGeneValidators() (*types.ValidatorSet, error) {
	validators := make([]*types.Validator, len(config.Config.GeneBftGroup))
	for i, val := range config.Config.GeneBftGroup {
		digestAddr := msp.AccountPubkeyToAddress1(msp.StringToBccspKey(val.PubKeyStr))
		validators[i] = &types.Validator{
			Address:   types.Address(digestAddr.GetBytes()),
			PubKeyStr: val.PubKeyStr,
			VotingPower: int64(val.Power),
		}
	}

	vals := make([]*types.Validator, len(validators))
	for i, val := range validators {
		vals[i] = val.Copy()
	}
	epochInfo := &Epoch{
		EpochNo:       1,
		DependEpochNo: 0,
		BeginNum:      1,
		EndNum:        1,
		TargetBlkNum:  TARGET_BLK_COUNT,
		WitnessList:   vals,
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.Marshal(epochInfo)
	if err != nil {
		ConsLog.Errorf(LOGTABLE_DBFT, "Marshal err %s", err.Error())
		return nil, err
	}

	vset := types.NewValidatorSet(validators, 1, nil, nil, data)
	index := len(validators) - 1
	if index >= 0 {
		vset.Proposer = validators[index]
	}
	return vset, nil

}
