package algorand

import (
	bsComm "HNB/ledger/blockStore/common"
	"HNB/consensus/algorand/state"
	"HNB/consensus/algorand/msgHandler"
	"HNB/consensus/algorand/types"
	"HNB/msp"
	"HNB/common"
	"HNB/ledger"
	"HNB/logging"
	"HNB/config"
	"HNB/txpool"

	"time"
	"fmt"
	"crypto/sha256"
)

var ConsLog logging.LogModule
const(
	LOGTABLE_CONS string = "consensus"
)

type Core struct {
	BftMgr      *msgHandler.BftMgr
}

type Server struct {
	*Core
}
func NewAlgorandServer() *Server{
	c,_ := NewCore()
	return &Server{c}
}


func (s *Server)Start(){
	err := s.Core.InitCons()
	if err != nil{
		panic(err.Error())
	}
}

func (s *Server)Stop(){

}


func NewCore() (*Core, error) {
	ConsLog = logging.GetLogIns()
	t := &Core{}
	return t, nil
}

func (cons *Core) InitCons() error {
	lastCommitState, err := cons.Init()
	if err != nil {
		panic("111" + err.Error())
		return err
	}

	handler, err := msgHandler.NewTDMMsgHandler(*lastCommitState)
	if err != nil {
		panic("222" + err.Error())
		return err
	}


	bftMgr, err := msgHandler.NewBftMgr(*lastCommitState)
	if err != nil {
		panic("333" + err.Error())
		return err
	}

	bftMgr.MsgHandler = handler
	cons.BftMgr = bftMgr

	bftMgr.Start()
	return nil
}

func (cons *Core) Init() (*state.State, error) {
	curH, err := ledger.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	ConsLog.Infof(LOGTABLE_CONS, "init get height %v", curH)
	if curH == 0 {
		genesisBlk, err := cons.MakeGenesisBlk()
		if err != nil {
			return nil, err
		}

		blk0, err := types.ConsToStandard(genesisBlk)
		if err != nil {
			return nil, err
		}

		err = ledger.WriteLedger(blk0, nil)
		//err = appMgr.BlockProcess(blk0)
		if err != nil {
			return nil, err
		}

		lastCommitState, err := cons.MakeGenesisLastCommitState(blk0)
		if err != nil {
			return nil, err
		}

		return lastCommitState, nil
	} else {
		blk, err := ledger.GetBlock(curH - 1)
		if err != nil {
			return nil, err
		}

		if blk == nil {
			return nil, fmt.Errorf("(tdmInit) blk %d is nil", curH-1)
		}
		lastCommitState, err := cons.LoadLastCommitStateFromBlk(blk)
		if err != nil {
			ConsLog.Errorf(LOGTABLE_CONS, "get last commit state err:%v", err.Error())
			return nil, err
		}

		return lastCommitState, nil
	}
}

func (cons *Core) LoadLastCommitStateFromBlk(block *bsComm.Block) (*state.State, error) {
	tdmBlk, err := types.Standard2Cons(block)
	if err != nil {
		return nil, err
	}
	curValidators, err := cons.LoadValidators(block)
	if err != nil {
		return nil, err
	}

	var lastValidators *types.ValidatorSet

	if block.Header.BlockNum == 0 {
		lastValidators = types.NewValidatorSet(nil, 0)
	} else {
		prevBlk, err := ledger.GetBlock(block.Header.BlockNum - 1)
		if prevBlk == nil {
			return nil, fmt.Errorf("tdm get last blk %d nil %v", block.Header.BlockNum - 1, err)
		}

		if err != nil {
			return nil, err
		}
		lastValidators, err = cons.LoadValidators(prevBlk)
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

func (cons *Core) MakeGenesisLastCommitState(blk *bsComm.Block) (*state.State, error) {
	validatorSet, err := cons.LoadGeneValidators()
	if err != nil {
		return nil, err
	}
	preblkHash, err := ledger.CalcBlockHash(blk)
	if err != nil {
		return nil, err
	}

	return &state.State{
		LastBlockNum:  0,
		LastBlockID:   types.BlockID{},
		LastBlockTime: config.Config.GenesisTime,
		Validators:    validatorSet,
		LastValidators:                   types.NewValidatorSet(nil, 0),
		LastHeightValidatorsChanged:      0,
		LastHeightConsensusParamsChanged: 0,
		PreviousHash:                     preblkHash,
	}, nil
}

func (cons *Core) LoadGeneValidators() (*types.ValidatorSet, error) {
	validators := make([]*types.Validator, len(config.Config.GeneBftGroup))
	for i, val := range config.Config.GeneBftGroup {

		digestAddr, err := msp.Hash256(msp.GetPubBytesFromStr(val.PubKeyStr))
		if err != nil {
			return nil, err
		}
		validators[i] = &types.Validator{
			Address:     types.Address(digestAddr),
			PubKeyStr:   val.PubKeyStr,
			//权重字段目前用不到
			VotingPower: int64(val.Power),
		}
	}

	return types.NewValidatorSet(validators, 0), nil
}

func (cons *Core) MakeGenesisBlk() (*types.Block, error) {
	validatorSet, err := cons.LoadGeneValidators()
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	hash.Write([]byte("genesis"))
	txid := fmt.Sprintf("%x", hash.Sum(nil)[:len(hash.Sum(nil))/2])
	tx := common.Transaction{
		Payload:   "genesis",
		Txid:      txid,
		Timestamp: 1505007000000,
		Type:      txpool.HGS,
	}

	var txs []*common.Transaction
	txs = append(txs, &tx)
	geneTDMTxs, err := types.Tx2TDMTx(txs)
	if err != nil {
		return nil, err
	}
	genesisiTime, _ := time.Parse("2006-01-02 15:04:05", "2018-05-14 00:00:00")
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

func updateState(s state.State, blockID types.BlockID, header *types.Header) (state.State, error) {
	prevValSet := s.Validators.Copy()
	nextValSet := prevValSet.Copy()
	// update the validator set with the latest abciResponses
	lastHeightValsChanged := s.LastHeightValidatorsChanged
	nextValSet.IncrementAccum(1)
	nextParams := s.ConsensusParams
	lastHeightParamsChanged := s.LastHeightConsensusParamsChanged
	return state.State{
		LastBlockNum:                     header.BlockNum,
		LastBlockTotalTx:                 s.LastBlockTotalTx + header.NumTxs,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		Validators:                       nextValSet,
		LastValidators:                   s.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
	}, nil

}

// 从块里加载validators
func (cons *Core) LoadValidators(blk *bsComm.Block) (valSet *types.ValidatorSet, err error) {
	tdmBlk, err := types.Standard2Cons(blk)
	if err != nil {
		return nil, err
	}

	valSet = tdmBlk.Validators

	if valSet == nil {
		lastValidatorChangeHeightBlk, err := ledger.GetBlock(tdmBlk.LastHeightChanged)
		if err != nil {
			return nil, err
		}

		if lastValidatorChangeHeightBlk == nil {
			return nil, fmt.Errorf(" blk %d is nil", tdmBlk.LastHeightChanged)
		}

		lastTdmBlk, err := types.Standard2Cons(lastValidatorChangeHeightBlk)
		if err != nil {
			return nil, err
		}

		if lastTdmBlk.Validators == nil {
			return nil, fmt.Errorf("%d load validators err", tdmBlk.LastHeightChanged)
		}

		valSet = lastTdmBlk.Validators
	}

	return valSet, nil
}
