package types

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
)

type RoundVoteSet struct {
	Prevotes   *VoteSet
	Precommits *VoteSet
}

var (
	GotVoteFromUnwantedRoundError = errors.New("Peer has sent a vote that does not match our round for more than one round")
)

/*
Keeps track of all VoteSets from round 0 to round 'round'.

Also keeps track of up to one RoundVoteSet greater than
'round' from each peer, to facilitate catchup syncing of commits.

A commit is +2/3 precommits for a block at a round,
but which round is not known in advance, so when a peer
provides a precommit for a round greater than mtx.round,
we create a new entry in roundVoteSets but also remember the
peer to prevent abuse.
We let each peer provide us with up to 2 unexpected "catchup" rounds.
One for their LastCommit round, and another for the official commit round.
*/
type HeightVoteSet struct {
	chainID string
	height  uint64
	valSet  *ValidatorSet

	mtx               sync.Mutex
	round             int32                // max tracked round
	roundVoteSets     map[int]RoundVoteSet // keys: [0...round]
	peerCatchupRounds map[string][]int32   // keys: peer.ID; values: at most 2 rounds
}

func NewBlkNumVoteSet(height uint64, valSet *ValidatorSet) *HeightVoteSet {
	hvs := &HeightVoteSet{}
	hvs.Reset(height, valSet)
	return hvs
}

func (hvs *HeightVoteSet) Reset(height uint64, valSet *ValidatorSet) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()

	hvs.height = height
	hvs.valSet = valSet
	hvs.roundVoteSets = make(map[int]RoundVoteSet)
	hvs.peerCatchupRounds = make(map[string][]int32)

	hvs.addRound(0)
	hvs.round = 0
}

func (hvs *HeightVoteSet) Height() uint64 {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.height
}

func (hvs *HeightVoteSet) Round() int32 {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.round
}

// Create more RoundVoteSets up to round.
func (hvs *HeightVoteSet) SetRound(round int32) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if hvs.round != 0 && (round < hvs.round+1) {
		fmt.Errorf("SetRound() must increment hvs.round")
	}
	for r := hvs.round + 1; r <= round; r++ {
		if _, ok := hvs.roundVoteSets[int(r)]; ok {
			continue // Already exists because peerCatchupRounds.
		}
		hvs.addRound(r)
	}
	hvs.round = round
}

func (hvs *HeightVoteSet) addRound(round int32) {
	if _, ok := hvs.roundVoteSets[int(round)]; ok {
		fmt.Errorf("addRound() for an existing round")
	}
	prevotes := NewVoteSet(hvs.height, round, VoteTypePrevote, hvs.valSet)
	precommits := NewVoteSet(hvs.height, round, VoteTypePrecommit, hvs.valSet)

	hvs.roundVoteSets[int(round)] = RoundVoteSet{
		Prevotes:   prevotes,
		Precommits: precommits,
	}
}

// Duplicate votes return added=false, err=nil.
// By convention, peerID is "" if origin is self.
func (hvs *HeightVoteSet) AddVote(vote *Vote, pubKeyID []byte) (added bool, err error) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !IsVoteTypeValid(vote.Type) {
		return
	}

	//获取本轮次的投票信息
	voteSet := hvs.getVoteSet(vote.Round, vote.Type)
	if voteSet == nil {
		//某个人投票的轮次数
		rndz := hvs.peerCatchupRounds[string(pubKeyID)]

		hvs.addRound(vote.Round)

		voteSet = hvs.getVoteSet(vote.Round, vote.Type)

		hvs.peerCatchupRounds[string(pubKeyID)] = append(rndz, vote.Round)
	}

	added, err = voteSet.AddVote(vote)
	return
}

func (hvs *HeightVoteSet) Prevotes(round int32) *VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, VoteTypePrevote)
}

func (hvs *HeightVoteSet) Precommits(round int32) *VoteSet {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	return hvs.getVoteSet(round, VoteTypePrecommit)
}

// Last round and blockID that has +2/3 prevotes for a particular block or nil.
// Returns -1 if no such round exists.
func (hvs *HeightVoteSet) POLInfo() (polRound int32, polBlockID BlockID) {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	for r := hvs.round; r >= 0; r-- {
		rvs := hvs.getVoteSet(r, VoteTypePrevote)
		if rvs == nil {
			continue
		}
		polBlockID, ok := rvs.TwoThirdsMajority()
		if ok {
			return r, polBlockID
		}
	}
	return -1, BlockID{}
}

func (hvs *HeightVoteSet) getVoteSet(round int32, type_ byte) *VoteSet {
	rvs, ok := hvs.roundVoteSets[int(round)]
	if !ok {
		return nil
	}
	switch type_ {
	case VoteTypePrevote:
		return rvs.Prevotes
	case VoteTypePrecommit:
		return rvs.Precommits
	default:
		fmt.Errorf("Unexpected vote type %X", type_)
		return nil
	}
}

func (hvs *HeightVoteSet) String() string {
	return hvs.StringIndented("")
}

func (hvs *HeightVoteSet) StringIndented(indent string) string {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	vsStrings := make([]string, 0, (len(hvs.roundVoteSets)+1)*2)
	// rounds 0 ~ hvs.round inclusive
	for round := 0; round <= int(hvs.round); round++ {
		voteSetString := hvs.roundVoteSets[round].Prevotes.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = hvs.roundVoteSets[round].Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	// all other peer catchup rounds
	for round, roundVoteSet := range hvs.roundVoteSets {
		if round <= int(hvs.round) {
			continue
		}
		voteSetString := roundVoteSet.Prevotes.StringShort()
		vsStrings = append(vsStrings, voteSetString)
		voteSetString = roundVoteSet.Precommits.StringShort()
		vsStrings = append(vsStrings, voteSetString)
	}
	return cmn.Fmt(`HeightVoteSet{H:%v R:0~%v
%s  %v
%s}`,
		hvs.height, hvs.round,
		indent, strings.Join(vsStrings, "\n"+indent+"  "),
		indent)
}

// If a peer claims that it has 2/3 majority for given blockKey, call this.
// NOTE: if there are too many peers, or too much peer churn,
// this can cause memory issues.
func (hvs *HeightVoteSet) SetPeerMaj23(round int32, type_ byte, pubKeyID []byte, blockID BlockID) error {
	hvs.mtx.Lock()
	defer hvs.mtx.Unlock()
	if !IsVoteTypeValid(type_) {
		return fmt.Errorf("SetPeerMaj23: Invalid vote type %v", type_)
	}
	voteSet := hvs.getVoteSet(round, type_)
	if voteSet == nil {
		return nil // something we don't know about yet
	}
	return voteSet.SetPeerMaj23(pubKeyID, blockID)
}
