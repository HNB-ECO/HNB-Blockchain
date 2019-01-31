package types

import (
	"HNB/bccsp"
	"HNB/consensus/algorand/common"
	"HNB/util"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const TimeFormat = "2006-01-02T15:04:05.000Z"
const LOGGER_NAME = common.LOGGER_NAME

type Address util.HexBytes

type PubKey bccsp.Key

type Signature interface {
	Equals(signature Signature) bool
}

type BitArray struct {
	mtx   sync.Mutex
	Bits  int      `json:"bits"`  // NOTE: persisted via reflect, must be exported
	Elems []uint64 `json:"elems"` // NOTE: persisted via reflect, must be exported
}
type CanonicalJSONBlockID struct {
	Hash        util.HexBytes              `json:"hash,omitempty"`
	PartsHeader CanonicalJSONPartSetHeader `json:"parts,omitempty"`
}

type CanonicalJSONPartSetHeader struct {
	Hash  util.HexBytes `json:"hash,omitempty"`
	Total int           `json:"total,omitempty"`
}

type CanonicalJSONProposal struct {
	ChainID          string                     `json:"@chain_id"`
	Type             string                     `json:"@type"`
	BlockPartsHeader CanonicalJSONPartSetHeader `json:"block_parts_header"`
	Height           int64                      `json:"height"`
	POLBlockID       CanonicalJSONBlockID       `json:"pol_block_id"`
	POLRound         int                        `json:"pol_round"`
	Round            int                        `json:"round"`
	Timestamp        string                     `json:"timestamp"`
}

type CanonicalJSONVote struct {
	ChainID   string               `json:"@chain_id"`
	Type      string               `json:"@type"`
	BlockID   CanonicalJSONBlockID `json:"block_id"`
	Height    uint64               `json:"height"`
	Round     int32                `json:"round"`
	Timestamp string               `json:"timestamp"`
	VoteType  byte                 `json:"type"`
}

type CanonicalJSONHeartbeat struct {
	ChainID          string  `json:"@chain_id"`
	Type             string  `json:"@type"`
	Height           int64   `json:"height"`
	Round            int     `json:"round"`
	Sequence         int     `json:"sequence"`
	ValidatorAddress Address `json:"validator_address"`
	ValidatorIndex   int     `json:"validator_index"`
}

//-----------------------------------
// Canonicalize the structs

func CanonicalBlockID(blockID BlockID) CanonicalJSONBlockID {
	return CanonicalJSONBlockID{
		Hash:        blockID.Hash,
		PartsHeader: CanonicalPartSetHeader(blockID.PartsHeader),
	}
}

func (addr Address) String() string {
	return strings.ToUpper(hex.EncodeToString(util.HexBytes(addr)))
}

func CanonicalPartSetHeader(psh PartSetHeader) CanonicalJSONPartSetHeader {
	return CanonicalJSONPartSetHeader{
		psh.Hash,
		psh.Total,
	}
}

func CanonicalVote(chainID string, vote *Vote) CanonicalJSONVote {
	return CanonicalJSONVote{
		ChainID:   chainID,
		Type:      "vote",
		BlockID:   CanonicalBlockID(vote.BlockID),
		Height:    vote.Height,
		Round:     vote.Round,
		Timestamp: CanonicalTime(vote.Timestamp),
		VoteType:  vote.Type,
	}
}

func CanonicalTime(t time.Time) string {
	// Note that sending time over amino resets it to
	// local time, we need to force UTC here, so the
	// signatures match
	return t.UTC().Format(TimeFormat)
}
