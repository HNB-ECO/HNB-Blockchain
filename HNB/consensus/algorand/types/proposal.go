package types

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidBlockPartSignature = errors.New("Error invalid block part signature")
	ErrInvalidBlockPartHash      = errors.New("Error invalid block part hash")
)

// Proposal defines a block proposal for the consensus.
// It refers to the block only by its PartSetHeader.
// It must be signed by the correct proposer for the given BlockNum/Round
// to be considered valid. It may depend on votes from a previous round,
// a so-called Proof-of-Lock (POL) round, as noted in the POLRound and POLBlockID.
type Proposal struct {
	Height           uint64        `json:"height"`
	Round            int32         `json:"round"`
	Timestamp        time.Time     `json:"timestamp"`
	BlockPartsHeader PartSetHeader `json:"block_parts_header"`
	POLRound         int32         `json:"pol_round"`    // -1 if null.
	POLBlockID       BlockID       `json:"pol_block_id"` // zero if null.
	Signature        []byte        `json:"signature"`
}

// NewProposal returns a new Proposal.
// If there is no POLRound, polRound should be -1.
func NewProposal(height uint64, round int32, blockPartsHeader PartSetHeader, polRound int32, polBlockID BlockID) *Proposal {
	return &Proposal{
		Height:           height,
		Round:            round,
		Timestamp:        time.Now().UTC(),
		BlockPartsHeader: blockPartsHeader,
		POLRound:         polRound,
		POLBlockID:       polBlockID,
	}
}

// String returns a string representation of the Proposal.
func (p *Proposal) String() string {
	return fmt.Sprintf("Proposal{%v/%v %v (%v,%v) %v @ %s}",
		p.Height, p.Round, p.BlockPartsHeader, p.POLRound,
		p.POLBlockID, p.Signature, CanonicalTime(p.Timestamp))
}

// SignBytes returns the Proposal bytes for signing
// 签名统一在tdmMsg里签
func (p *Proposal) SignBytes(chainID string) []byte {
	//bz, err := cdc.MarshalJSON(CanonicalProposal(chainID, p))
	//if err != nil {
	//	panic(err)
	//}
	return nil
}
