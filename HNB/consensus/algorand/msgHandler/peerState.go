package msgHandler

import (
	"fmt"
	"sync"
	"time"
	cmn "HNB/consensus/algorand/common"
	"HNB/consensus/algorand/types"
)

// PeerState contains the known state of a peer, including its connection and
// threadsafe access to its PeerRoundState.
type PeerState struct {
	mtx    sync.Mutex
	types.PeerRoundState
	stats *peerStateStats
}

// peerStateStats holds internal statistics for a peer.
type peerStateStats struct {
	lastVoteHeight uint64
	votes          int

	lastBlockPartHeight uint64
	blockParts          int
}

func (pss peerStateStats) String() string {
	return fmt.Sprintf("peerStateStats{votes: %d, blockParts: %d}", pss.votes, pss.blockParts)
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState() *PeerState {
	return &PeerState{
		PeerRoundState: types.PeerRoundState{
			Round:              -1,
			ProposalPOLRound:   -1,
			LastCommitRound:    -1,
			CatchupCommitRound: -1,
		},
		stats: &peerStateStats{},
	}
}

// GetRoundState returns an shallow copy of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRountate() *types.PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PeerRoundState // copy
	return &prs
}

// GetRoundStateJSON returns a json of PeerRoundState, marshalled using go-amino.
func (ps *PeerState) GetRoundStateJSON() ([]byte, error) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return types.Codec.MarshalJSON(ps.PeerRoundState)
}

// GetHeight returns an atomic snapshot of the PeerRoundState's height
// used by the mempool to ensure peers are caught up before broadcasting new txs
func (ps *PeerState) GetHeight() uint64 {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.PeerRoundState.Height
}

// SetHasProposal sets the given proposal as known for the peer.
func (ps *PeerState) SetHasProposal(proposal *types.Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	//if ps.BlockNum != proposal.BlockNum || ps.Round != proposal.Round {
	//
	//	return
	//}
	if ps.Proposal {
		return
	}

	ps.Proposal = true
	ps.ProposalBlockPartsHeader = proposal.BlockPartsHeader
	ps.ProposalBlockParts = cmn.NewBitArray(proposal.BlockPartsHeader.Total)
	ps.ProposalPOLRound = proposal.POLRound
	ps.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

// InitProposalBlockParts initializes the peer's proposal block parts header and bit array.
func (ps *PeerState) InitProposalBlockParts(partsHeader types.PartSetHeader) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.ProposalBlockParts != nil {
		return
	}

	ps.ProposalBlockPartsHeader = partsHeader
	ps.ProposalBlockParts = cmn.NewBitArray(partsHeader.Total)
}

// SetHasProposalBlockPart sets the given block part index as known for the peer.
func (ps *PeerState) SetHasProposalBlockPart(height uint64, round int32, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	//if ps.BlockNum != height || ps.Round != round {
	//	return
	//}
	ps.ProposalBlockParts.SetIndex(index, true)
}

// PickSendVote picks a vote and sends it to the peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) bool {
	// TODO 投票
	/*if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}
		ps.logger.Debug("Sending vote message", "ps", ps, "vote", vote)
		return ps.Peer.Send(VoteChannel, cdc.MustMarshalBinaryBare(msg))
	}*/
	return false
}

// PickVoteToSend picks a vote to send to the peer.
// Returns true if a vote was picked.
// NOTE: `votes` must be the correct Size() for the BlockNum().
func (ps *PeerState) PickVoteToSend(votes types.VoteSetReader) (vote *types.Vote, ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if votes.Size() == 0 {
		return nil, false
	}

	height, round, type_, size := votes.Height(), votes.Round(), votes.Type(), votes.Size()

	// Lazily set data using 'votes'.
	if votes.IsCommit() {
		ps.ensureCatchupCommitRound(height, round, size)
	}
	ps.ensureVoteBitArrays(height, size)

	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes == nil {
		return nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		ps.setHasVote(height, round, type_, index)
		return votes.GetByIndex(index), true
	}
	return nil, false
}

func (ps *PeerState) getVoteBitArray(height uint64, round int32, type_ byte) *cmn.BitArray {
	if !types.IsVoteTypeValid(type_) {
		return nil
	}

	if ps.Height == height {
		if ps.Round == round {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.Prevotes
			case types.VoteTypePrecommit:
				return ps.Precommits
			}
		}
		if ps.CatchupCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.CatchupCommit
			}
		}
		if ps.ProposalPOLRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.ProposalPOL
			case types.VoteTypePrecommit:
				return nil
			}
		}
		return nil
	}
	if ps.Height == height+1 {
		if ps.LastCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.LastCommit
			}
		}
		return nil
	}
	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height uint64, round int32, numValidators int) {
	if ps.Height != height {
		return
	}
	/*
		NOTE: This is wrong, 'round' could change.
		e.g. if orig round is not the same as block LastCommit round.
		if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
			cmn.PanicSanity(cmn.Fmt("Conflicting CatchupCommitRound. BlockNum: %v, Orig: %v, New: %v", height, ps.CatchupCommitRound, round))
		}
	*/
	if ps.CatchupCommitRound == round {
		return // Nothing to do!
	}
	ps.CatchupCommitRound = round
	if round == ps.Round {
		ps.CatchupCommit = ps.Precommits
	} else {
		ps.CatchupCommit = cmn.NewBitArray(numValidators)
	}
}

// EnsureVoteBitArrays ensures the bit-arrays have been allocated for tracking
// what votes this peer has received.
// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height uint64, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height uint64, numValidators int) {
	if ps.Height == height {
		if ps.Prevotes == nil {
			ps.Prevotes = cmn.NewBitArray(numValidators)
		}
		if ps.Precommits == nil {
			ps.Precommits = cmn.NewBitArray(numValidators)
		}
		if ps.CatchupCommit == nil {
			ps.CatchupCommit = cmn.NewBitArray(numValidators)
		}
		if ps.ProposalPOL == nil {
			ps.ProposalPOL = cmn.NewBitArray(numValidators)
		}
	} else if ps.Height == height+1 {
		if ps.LastCommit == nil {
			ps.LastCommit = cmn.NewBitArray(numValidators)
		}
	}
}

// RecordVote updates internal statistics for this peer by recording the vote.
// It returns the total number of votes (1 per block). This essentially means
// the number of blocks for which peer has been sending us votes.
func (ps *PeerState) RecordVote(vote *types.Vote) int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.stats.lastVoteHeight >= vote.Height {
		return ps.stats.votes
	}
	ps.stats.lastVoteHeight = vote.Height
	ps.stats.votes++
	return ps.stats.votes
}

// VotesSent returns the number of blocks for which peer has been sending us
// votes.
func (ps *PeerState) VotesSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.stats.votes
}

// BlockPartsSent returns the number of blocks for which peer has been sending
// us block parts.
func (ps *PeerState) BlockPartsSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.stats.blockParts
}

// SetHasVote sets the given vote as known by the peer
func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, vote.Round, vote.Type, int(vote.ValidatorIndex)) //TODO  转int了
}

func (ps *PeerState) setHasVote(height uint64, round int32, type_ byte, index int) {
	// NOTE: some may be nil BitArrays -> no side effects.
	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes != nil {
		psVotes.SetIndex(index, true)
	}
}

// ApplyNewRoundStepMessage updates the peer state for the new round.
func (ps *PeerState) ApplyNewRoundStepMessage(msg *cmn.NewRoundStepMessage) {
	ConsLog.Infof(LOGTABLE_CONS, "ApplyNewRoundStepMessage msg *tdmProto.NewRoundStepMessage msg 是:", msg)
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Ignore duplicates or decreases
	if CompareHRS(msg.Height, msg.Round, types.RoundStepType(msg.Step), ps.Height, ps.Round, ps.Step) <= 0 {
		return
	}

	// Just remember these values.
	psHeight := ps.Height
	psRound := ps.Round
	//psStep := ps.Step
	psCatchupCommitRound := ps.CatchupCommitRound
	psCatchupCommit := ps.CatchupCommit

	startTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.Height = msg.Height
	ps.Round = msg.Round
	ps.Step = types.RoundStepType(msg.Step)
	ps.StartTime = startTime
	ConsLog.Infof(LOGTABLE_CONS, "ApplyNewRoundStepMessage: psHeight != msg.BlockNum || psRound != msg.Round", psHeight != msg.Height || psRound != msg.Round)
	if psHeight != msg.Height || psRound != msg.Round {
		ps.Proposal = false
		ps.ProposalBlockPartsHeader = types.PartSetHeader{}
		ps.ProposalBlockParts = nil
		ps.ProposalPOLRound = -1
		ps.ProposalPOL = nil
		// We'll update the BitArray capacity later.
		ps.Prevotes = nil
		ps.Precommits = nil
	}
	if psHeight == msg.Height && psRound != msg.Round && msg.Round == psCatchupCommitRound {
		// Peer caught up to CatchupCommitRound.
		// Preserve psCatchupCommit!
		// NOTE: We prefer to use prs.Precommits if
		// pr.Round matches pr.CatchupCommitRound.
		ps.Precommits = psCatchupCommit
	}
	if psHeight != msg.Height {
		// Shift Precommits to LastCommit.
		if psHeight+1 == msg.Height && psRound == msg.LastCommitRound {
			ps.LastCommitRound = msg.LastCommitRound
			ps.LastCommit = ps.Precommits
		} else {
			ps.LastCommitRound = msg.LastCommitRound
			ps.LastCommit = nil
		}
		// We'll update the BitArray capacity later.
		ps.CatchupCommitRound = -1
		ps.CatchupCommit = nil
	}
}

//// ApplyCommitStepMessage updates the peer state for the new commit.
//func (ps *PeerState) ApplyCommitStepMessage(msg *CommitStepMessage) {
//	ps.mtx.Lock()
//	defer ps.mtx.Unlock()
//
//	if ps.BlockNum != msg.BlockNum {
//		return
//	}
//
//	ps.ProposalBlockPartsHeader = msg.BlockPartsHeader
//	ps.ProposalBlockParts = msg.BlockParts
//}
//
// ApplyProposalPOLMessage updates the peer state for the new proposal POL.
func (ps *PeerState) ApplyProposalPOLMessage(msg *cmn.ProposalPOLMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}
	if ps.ProposalPOLRound != msg.ProposalPOLRound {
		return
	}

	// We might have sent some prevotes in the meantime.
	bitArray := &cmn.BitArray{
		Bits:  int(msg.ProposalPOL.Bits),
		Elems: msg.ProposalPOL.Elems,
	}
	ps.ProposalPOL = bitArray
}


// String returns a string representation of the PeerState
func (ps *PeerState) String() string {
	return ps.StringIndented("")
}

// StringIndented returns a string representation of the PeerState
func (ps *PeerState) StringIndented(indent string) string {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf(`PeerState{
%s  PRS   %v
%s  Stats %v
%s}`,
		indent, ps.PeerRoundState.StringIndented(indent+"  "),
		indent, ps.stats,
		indent)
}

func CompareHRS(h1 uint64, r1 int32, s1 types.RoundStepType, h2 uint64, r2 int32, s2 types.RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}
