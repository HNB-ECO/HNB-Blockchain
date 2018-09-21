package types

import (
	"HNB/bccsp"
	"HNB/bccsp/sw"
	"HNB/common"
	bsComm "HNB/ledger/blockStore/common"
	"HNB/msp"
	"crypto/elliptic"
	"encoding/json"
	"github.com/tendermint/go-amino"
)

var Codec = amino.NewCodec()

// todo 这里需要加上支持的公私钥具体类型
func init() {
	Codec.RegisterInterface((*PubKey)(nil), nil)
	Codec.RegisterInterface((*bccsp.Key)(nil), nil)
	Codec.RegisterInterface((*elliptic.Curve)(nil), nil)
	Codec.RegisterConcrete(&elliptic.CurveParams{}, "bccsp/CurveParams", nil)
	Codec.RegisterConcrete(&sw.EcdsaPublicKey{}, "bccsp/ECDSAPublicKey", nil)
	Codec.RegisterConcrete(&sw.EcdsaPrivateKey{}, "bccsp/ECDSAPrivateKey", nil)
	//Codec.RegisterConcrete(&bccsp.ECDSAPublicKey{}, "bccsp/ECDSAPublicKey", nil)
	//Codec.RegisterConcrete(&bccsp.ECDSAPrivateKey{}, "bccsp/ECDSAPrivateKey", nil)
	//Codec.RegisterConcrete(&bccsp.SM2PublicKey{}, "bccsp/SM2PublicKey", nil)
	//Codec.RegisterConcrete(&bccsp.SM2PrivateKey{}, "bccsp/SM2PrivateKey", nil)

}

func ConsToStandard(block *Block) (*bsComm.Block, error) {

	customHeader := &CustomTDMHeader{
		LastCommit:    block.LastCommit,
		DataHash:      block.Data.hash,
		Evidence:      block.Evidence,
		Header:        block.Header,
		ValidatorInfo: block.ValidatorInfo,
	}

	CustomTDMExt := &CustomTDMExt{
		CurrentCommit: block.CurrentCommit,
	}
	customHeaderData, err := Codec.MarshalBinary(customHeader)
	if err != nil {
		return nil, err
	}

	CustomTDMExtData, err := Codec.MarshalBinary(CustomTDMExt)
	if err != nil {
		return nil, err
	}

	txNum := block.Header.NumTxs
	uniformTxs := make([]*common.Transaction, txNum)
	if txNum != 0 {
		var i int64
		for i = 0; i < txNum; i++ {
			txByte := block.Data.Txs[i]
			uniformTx := &common.Transaction{}
			err := json.Unmarshal(txByte, uniformTx)
			if err != nil {
				return nil, err
			}

			uniformTxs[i] = uniformTx
		}
	}

	//todo 需要计算默克尔数Hash
	txsHash, err := CalcTxsHash(uniformTxs)
	if err != nil {
		return nil, err
	}

	var proposer *Validator
	if block.BlockNum != 0 {
		proposer = block.Proposer
	} else {
		proposer = &Validator{}
	}

	header := &bsComm.Header{
		BlockNum:     block.BlockNum,
		CreateTime:   uint64(block.Time.UnixNano() / 1000000),
		PreviousHash: make([]byte, 0),
		ConsArgs:     customHeaderData,
		Ext:          CustomTDMExtData,
		TxsHash:      txsHash,
		SenderId:     proposer.Address,
	}

	retBlk := &bsComm.Block{
		Header: header,
		Txs:    uniformTxs,
	}

	return retBlk, nil
}

// 计算交易集的hash
func CalcTxsHash(txs []*common.Transaction) ([]byte, error) {
	txsBytes, err := json.Marshal(txs)
	if err != nil {
		return nil, err
	}

	return msp.Hash256(txsBytes)
}

func Standard2Cons(blk *bsComm.Block) (block *Block, err error) {
	customHeader := &CustomTDMHeader{}
	customTDMExt := &CustomTDMExt{}

	err = Codec.UnmarshalBinary(blk.Header.ConsArgs, customHeader)
	if err != nil {
		return nil, err
	}

	err = Codec.UnmarshalBinary(blk.Header.Ext, customTDMExt)
	if err != nil {
		return nil, err
	}

	txNum := len(blk.Txs)
	tdmTxs := make([]Tx, txNum)
	if txNum != 0 {
		for i := 0; i < txNum; i++ {
			tx := blk.Txs[i]
			txByte, err := json.Marshal(tx)
			if err != nil {
				return nil, err
			}

			tdmTxs[i] = Tx(txByte)
		}
	}

	retBlk := &Block{
		Evidence:      customHeader.Evidence,
		LastCommit:    customHeader.LastCommit,
		CurrentCommit: customTDMExt.CurrentCommit,
		Header:        customHeader.Header,
		Data: &Data{
			Txs:  tdmTxs,
			hash: customHeader.DataHash,
		},
		ValidatorInfo: customHeader.ValidatorInfo,
	}

	return retBlk, nil
}

func Tx2TDMTx(uniformTxs []*common.Transaction) ([]Tx, error) {
	txNum := len(uniformTxs)
	tdmTxs := []Tx{}

	if txNum != 0 {
		for i := 0; i < txNum; i++ {
			tx := uniformTxs[i]
			if tx == nil {
				continue
			} else {
				txByte, err := json.Marshal(tx)
				if err != nil {
					return nil, err
				}
				tdmTxs = append(tdmTxs, Tx(txByte))
			}
		}
	}

	return tdmTxs, nil
}
