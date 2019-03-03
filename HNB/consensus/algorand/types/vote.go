package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/util"
	"github.com/pkg/errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrVoteUnexpectedStep            = errors.New("Unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("Invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("Invalid validator address")
	ErrVoteInvalidSignature          = errors.New("Invalid signature")
	ErrVoteInvalidBlockHash          = errors.New("Invalid block hash")
	ErrVoteNonDeterministicSignature = errors.New("Non-deterministic signature")
	ErrVoteNil                       = errors.New("Nil vote")
	ErrVoteConfict                   = errors.New("Confict vote")
)

type ErrVoteConflictingVotes struct {
	err error
}

func NewConflictingVoteError(val *Validator, voteA, voteB *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{err: ErrVoteConfict}
}

// Types of votes
const (
	VoteTypePrevote   = byte(0x01)
	VoteTypePrecommit = byte(0x02)
)

func IsVoteTypeValid(type_ byte) bool {
	switch type_ {
	case VoteTypePrevote:
		return true
	case VoteTypePrecommit:
		return true
	default:
		return false
	}
}

type Vote struct {
	ValidatorAddress Address       `json:"validator_address"`
	ValidatorIndex   int           `json:"validator_index"`
	Height           uint64        `json:"height"`
	Round            int32         `json:"round"`
	Timestamp        time.Time     `json:"timestamp"`
	Type             byte          `json:"type"`
	BlockID          BlockID       `json:"block_id"` // zero if vote is nil.
	Signature        util.HexBytes `json:"signature"`
}

func (vote *Vote) SignBytes(chainID string) []byte {
	bz, err := json.Marshal(CanonicalVote(chainID, vote))
	if err != nil {
		panic(err)
	}
	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	return &voteCopy
}

func (vote *Vote) String() string {
	if vote == nil {
		return "nil-Vote"
	}
	var typeString string
	switch vote.Type {
	case VoteTypePrevote:
		typeString = "Prevote"
	case VoteTypePrecommit:
		typeString = "Precommit"
	default:
		cmn.PanicSanity("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X  @ %s}",
		vote.ValidatorIndex, cmn.Fingerprint(vote.ValidatorAddress),
		vote.Height, vote.Round, vote.Type, typeString,
		cmn.Fingerprint(vote.BlockID.Hash),
		CanonicalTime(vote.Timestamp))
}

func (vote *Vote) VoteString() string {
	if vote == nil {
		return "nil-Vote"
	}
	var vString string
	switch vote.Type {
	case VoteTypePrevote:
		vString = "Prevote"
	case VoteTypePrecommit:
		vString = "Precommit"
	default:
		vString = "Unknown vote type"
	}
	return vString
}

type VoteSet struct {
	chainID       string
	height        uint64
	round         int32
	type_         byte
	mtx           sync.Mutex
	valSet        *ValidatorSet
	votesBitArray *cmn.BitArray
	votes         []*Vote                // Primary votes to share
	sum           int64                  // Sum of voting power for seen votes, discounting conflicts
	maj23         *BlockID               // First 2/3 majority seen
	votesByBlock  map[string]*blockVotes // string(blockHash|blockParts) -> blockVotes
	peerMaj23s    map[string]BlockID     // Maj23 for each peer
	//log           *logging.Logger
}

// Constructs a new VoteSet struct used to accumulate votes for given height/round.
func NewVoteSet(height uint64, round int32, type_ byte, valSet *ValidatorSet) *VoteSet {
	if height == 0 {
		cmn.PanicSanity("Cannot make VoteSet for height == 0, doesn't make sense.")
	}
	return &VoteSet{
		height:        height,
		round:         round,
		type_:         type_,
		valSet:        valSet,
		votesBitArray: cmn.NewBitArray(valSet.Size()),
		votes:         make([]*Vote, valSet.Size()),
		sum:           0,
		maj23:         nil,
		votesByBlock:  make(map[string]*blockVotes, valSet.Size()),
		peerMaj23s:    make(map[string]BlockID),
	}
}

func (voteSet *VoteSet) ChainID() string {
	return voteSet.chainID
}

func (voteSet *VoteSet) Height() uint64 {
	if voteSet == nil {
		return 0
	}
	return voteSet.height
}

func (voteSet *VoteSet) Round() int32 {
	if voteSet == nil {
		return -1
	}
	return voteSet.round
}

func (voteSet *VoteSet) Type() byte {
	if voteSet == nil {
		return 0x00
	}
	return voteSet.type_
}

func (voteSet *VoteSet) Size() int {
	if voteSet == nil {
		return 0
	}
	return voteSet.valSet.Size()
}

// Returns added=true if vote is valid and new.
// Otherwise returns err=ErrVote[
//		UnexpectedStep | InvalidIndex | InvalidAddress |
//		InvalidSignature | InvalidBlockHash | ConflictingVotes ]
// Duplicate votes return added=false, err=nil.
// Conflicting votes return added=*, err=ErrVoteConflictingVotes.
// NOTE: vote should not be mutated after adding.
// NOTE: VoteSet must not be nil
// NOTE: Vote must not be nil
func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error) {
	if voteSet == nil {
		cmn.PanicSanity("AddVote() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	return voteSet.addVote(vote)
}

// NOTE: Validates as much as possible before attempting to verify the signature.
func (voteSet *VoteSet) addVote(vote *Vote) (added bool, err error) {
	if vote == nil {
		return false, ErrVoteNil
	}
	valIndex := vote.ValidatorIndex
	valAddr := vote.ValidatorAddress
	blockKey := vote.BlockID.Key()

	// Ensure that validator index was set
	if valIndex < 0 {
		return false, errors.Wrap(ErrVoteInvalidValidatorIndex, "Index < 0")
	} else if len(valAddr) == 0 {
		return false, errors.Wrap(ErrVoteInvalidValidatorAddress, "Empty address")
	}

	// Make sure the step matches.
	if (vote.Height != voteSet.height) ||
		(vote.Round != voteSet.round) ||
		(vote.Type != voteSet.type_) {
		return false, errors.Wrapf(ErrVoteUnexpectedStep, "Got %d/%d/%d, expected %d/%d/%d",
			voteSet.height, voteSet.round, voteSet.type_,
			vote.Height, vote.Round, vote.Type)
	}

	// Ensure that signer is a validator.
	lookupAddr, val := voteSet.valSet.GetByIndex(int(valIndex))
	if val == nil {
		return false, errors.Wrapf(ErrVoteInvalidValidatorIndex,
			"Cannot find validator %d in valSet of size %d", valIndex, voteSet.valSet.Size())
	}

	// Ensure that the signer has the right address
	if !bytes.Equal(valAddr, lookupAddr) {
		return false, errors.Wrapf(ErrVoteInvalidValidatorAddress,
			"vote.ValidatorAddress (%X) does not match address (%X) for vote.ValidatorIndex (%d)\nEnsure the genesis file is correct across all validators.",
			valAddr, lookupAddr, valIndex)
	}

	// If we already know of this vote, return false.
	if existing, ok := voteSet.getVote(int(valIndex), blockKey); ok {
		//if existing.Signature.Equals(vote.Signature) {
		//	return false, nil // duplicate
		//}

		return false, errors.Wrapf(ErrVoteNonDeterministicSignature, "Existing vote: %v; New vote: %v", existing, vote)
	}

	// Check signature.
	//if err := vote.Verify(voteSet.chainID, val.PubKey); err != nil {
	//	return false, errors.Wrapf(err, "Failed to verify vote with ChainID %s and PubKey %s", voteSet.chainID, val.PubKey)
	//}

	// Add vote and get conflicting vote if any
	added, conflicting := voteSet.addVerifiedVote(vote, blockKey, val.VotingPower)
	if conflicting != nil {
		return added, NewConflictingVoteError(val, conflicting, vote).err
	}
	if !added {
		cmn.PanicSanity("Expected to add non-conflicting vote")
	}
	return added, nil
}

// Returns (vote, true) if vote exists for valIndex and blockKey
func (voteSet *VoteSet) getVote(valIndex int, blockKey string) (vote *Vote, ok bool) {
	if existing := voteSet.votes[valIndex]; existing != nil && existing.BlockID.Key() == blockKey {
		return existing, true
	}
	if existing := voteSet.votesByBlock[blockKey].getByIndex(valIndex); existing != nil {
		return existing, true
	}
	return nil, false
}

func (voteSet *VoteSet) addVerifiedVote(vote *Vote, blockKey string, votingPower int64) (added bool, conflicting *Vote) {

	valIndex := vote.ValidatorIndex
	votingPower = 1 // 设置1
	//验证者是否已经投过票了
	if existing := voteSet.votes[valIndex]; existing != nil {
		if existing.BlockID.Equals(vote.BlockID) {
			cmn.PanicSanity("addVerifiedVote does not expect duplicate votes")
		} else {
			conflicting = existing
		}
		// Replace vote if blockKey matches voteSet.maj23.
		if voteSet.maj23 != nil && voteSet.maj23.Key() == blockKey {
			voteSet.votes[valIndex] = vote
			voteSet.votesBitArray.SetIndex(int(valIndex), true)
		}
		// Otherwise don't add it to voteSet.votes
	} else {
		//记录投票信息
		voteSet.votes[valIndex] = vote
		voteSet.votesBitArray.SetIndex(int(valIndex), true)
		voteSet.sum += votingPower
	}
	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if conflicting != nil && !votesByBlock.peerMaj23 {
			return false, conflicting
		}
	} else {
		if conflicting != nil {
			return false, conflicting
		}
		votesByBlock = newBlockVotes(false, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
	}

	origSum := votesByBlock.sum
	quorum := voteSet.valSet.TotalVotingPower()*2/3 + 1
	votesByBlock.addVerifiedVote(vote, votingPower)
	if origSum < quorum && quorum <= votesByBlock.sum {
		// Only consider the first quorum reached
		if voteSet.maj23 == nil {
			maj23BlockID := vote.BlockID
			voteSet.maj23 = &maj23BlockID
			// And also copy votes over to voteSet.votes
			for i, vote := range votesByBlock.votes {
				if vote != nil {
					voteSet.votes[i] = vote
				}
			}
		}
	}

	return true, conflicting
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
// NOTE: VoteSet must not be nil
func (voteSet *VoteSet) SetPeerMaj23(peerID []byte, blockID BlockID) error {
	if voteSet == nil {
		cmn.PanicSanity("SetPeerMaj23() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	blockKey := blockID.Key()

	// Make sure peer hasn't already told us something.
	if existing, ok := voteSet.peerMaj23s[string(peerID)]; ok {
		if existing.Equals(blockID) {
			return nil // Nothing to do
		}
		return fmt.Errorf("SetPeerMaj23: Received conflicting blockID from peer %v. Got %v, expected %v",
			peerID, blockID, existing)
	}
	voteSet.peerMaj23s[string(peerID)] = blockID

	// Create .votesByBlock entry if needed.
	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if votesByBlock.peerMaj23 {
			return nil // Nothing to do
		}
		votesByBlock.peerMaj23 = true
		// No need to copy votes, already there.
	} else {
		votesByBlock = newBlockVotes(true, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// No need to copy votes, no votes to copy over.
	}
	return nil
}

func (voteSet *VoteSet) BitArray() *cmn.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votesBitArray.Copy()
}

func (voteSet *VoteSet) BitArrayByBlockID(blockID BlockID) *cmn.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	votesByBlock, ok := voteSet.votesByBlock[blockID.Key()]
	if ok {
		return votesByBlock.bitArray.Copy()
	}
	return nil
}

// NOTE: if validator has conflicting votes, returns "canonical" vote
func (voteSet *VoteSet) GetByIndex(valIndex int) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) GetByAddress(address []byte) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	valIndex, val := voteSet.valSet.GetByAddress(address)
	if val == nil {
		cmn.PanicSanity("GetByAddress(address) returned nil")
	}
	return voteSet.votes[valIndex]
}

func (voteSet *VoteSet) HasTwoThirdsMajority() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) IsCommit() bool {
	if voteSet == nil {
		return false
	}
	if voteSet.type_ != VoteTypePrecommit {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) HasTwoThirdsAny() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum > voteSet.valSet.TotalVotingPower()*2/3
}

func (voteSet *VoteSet) HasHalfAny() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	//fmt.Printf("sum:", voteSet.sum, ",Total:", voteSet.valSet.TotalVotingPower())
	return voteSet.sum > voteSet.valSet.TotalVotingPower()*1/3
}

func (voteSet *VoteSet) Detail() string {
	return fmt.Sprintf("sum %d, totalVotingPower %d", voteSet.sum, voteSet.valSet.TotalVotingPower())
}

func (voteSet *VoteSet) HasAll() bool {
	return voteSet.sum == voteSet.valSet.TotalVotingPower()
}

// Returns either a blockhash (or nil) that received +2/3 majority.
// If there exists no such majority, returns (nil, PartSetHeader{}, false).
func (voteSet *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool) {
	if voteSet == nil {
		return BlockID{}, false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if voteSet.maj23 != nil {
		return *voteSet.maj23, true
	}
	return BlockID{}, false
}

func (voteSet *VoteSet) String() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	return voteSet.StringIndented("")
}

func (voteSet *VoteSet) StringIndented(indent string) string {
	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = "nil-Vote"
		} else {
			voteStrings[i] = vote.String()
		}
	}
	return fmt.Sprintf(`VoteSet{
%s  H:%v R:%v T:%v
%s  %v
%s  %v
%s  %v
%s}`,
		indent, voteSet.height, voteSet.round, voteSet.type_,
		indent, strings.Join(voteStrings, "\n"+indent+"  "),
		indent, voteSet.votesBitArray,
		indent, voteSet.peerMaj23s,
		indent)
}

func (voteSet *VoteSet) StringShort() string {
	if voteSet == nil {
		return "nil-VoteSet"
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return fmt.Sprintf(`VoteSet{H:%v R:%v T:%v +2/3:%v %v %v}`,
		voteSet.height, voteSet.round, voteSet.type_, voteSet.maj23, voteSet.votesBitArray, voteSet.peerMaj23s)
}

//--------------------------------------------------------------------------------
// Commit

func (voteSet *VoteSet) MakeCommit() *Commit {
	if voteSet.type_ != VoteTypePrecommit {
		cmn.PanicSanity("Cannot MakeCommit() unless VoteSet.Type is VoteTypePrecommit")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	// Make sure we have a 2/3 majority
	if voteSet.maj23 == nil {
		cmn.PanicSanity("Cannot MakeCommit() unless a blockhash has +2/3")
	}

	// For every validator, get the precommit
	votesCopy := make([]*Vote, len(voteSet.votes))
	copy(votesCopy, voteSet.votes)
	return &Commit{
		BlockID:    *voteSet.maj23,
		Precommits: votesCopy,
	}
}

type blockVotes struct {
	peerMaj23 bool          // peer claims to have maj23
	bitArray  *cmn.BitArray // valIndex -> hasVote?
	votes     []*Vote       // valIndex -> *Vote
	sum       int64         // vote sum
}

func newBlockVotes(peerMaj23 bool, numValidators int) *blockVotes {
	return &blockVotes{
		peerMaj23: peerMaj23,
		bitArray:  cmn.NewBitArray(numValidators),
		votes:     make([]*Vote, numValidators),
		sum:       0,
	}
}

func (vs *blockVotes) addVerifiedVote(vote *Vote, votingPower int64) {
	valIndex := vote.ValidatorIndex
	if existing := vs.votes[valIndex]; existing == nil {
		vs.bitArray.SetIndex(int(valIndex), true)
		vs.votes[valIndex] = vote
		vs.sum += votingPower
	}

}

func (vs *blockVotes) getByIndex(index int) *Vote {
	if vs == nil {
		return nil
	}
	return vs.votes[index]
}

//--------------------------------------------------------------------------------

// Common interface between *consensus.VoteSet and types.Commit
type VoteSetReader interface {
	Height() uint64
	Round() int32
	Type() byte
	Size() int
	BitArray() *cmn.BitArray
	GetByIndex(int) *Vote
	IsCommit() bool
}
