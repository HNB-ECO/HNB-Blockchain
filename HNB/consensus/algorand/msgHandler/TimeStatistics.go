package msgHandler

import (
	"HNB/consensus/algorand/types"
	"fmt"
	"time"
)
type TimeStatistic struct {
	newRoundStart time.Time
	newRoundEnd   float64

	prevoteStart time.Time
	prevoteEnd   float64

	preposeStart time.Time
	preposeEnd   float64

	precommitStart time.Time
	precommitEnd   float64

	commitStart time.Time
	commitEnd   float64

	processStart time.Time
	processEnd   float64
	chainID      string
	height       uint64
	round        int32

	initTime  int64
	startTime int64
	endTime   int64

	enterNewRoundStart  int64
	enterProposalStart  int64
	enterPreVoteStart   int64
	enterPreCommitStart int64

	beforeWriteBlock int64
	afterWriteBlock  int64

	beforeGetFrontOriginalBatch int64
	afterGetFrontOriginalBatch  int64

	extraMapArr []map[string]int64

	txNum      int
	totaltxNum int
}

//进入 newRound 重置时间
func (t *TimeStatistic) ResetConsumeTime(h *TDMMsgHandler) {
	if h.Step == types.RoundStepNewHeight {
		t.height = h.Height
		t.newRoundStart = time.Now()
		t.newRoundEnd = 0
		t.preposeEnd = 0
		t.prevoteEnd = 0
		t.precommitEnd = 0
		t.processEnd = 0
		t.commitEnd = 0
	} else if h.Step == types.RoundStepNewRound {
		t.preposeStart = time.Now()
	} else if h.Step == types.RoundStepPropose {
		t.prevoteStart = time.Now()
	} else if h.Step == types.RoundStepPrevote {
		t.precommitStart = time.Now()
	} else if h.Step == types.RoundStepPrecommit || h.Step == types.RoundStepPrecommitWait {
		t.commitStart = time.Now()
	} else if h.Step == types.RoundStepCommit {
		t.processStart = time.Now()
	}
}

//开始超时时间设置
func (t *TimeStatistic) EndConsumeTime(h *TDMMsgHandler) {

	if t.height == h.Height && h.Step == types.RoundStepNewRound {
		t.newRoundEnd = time.Now().Sub(t.newRoundStart).Seconds()
	} else if t.height == h.Height && h.Step == types.RoundStepPropose {
		t.preposeEnd = time.Now().Sub(t.preposeStart).Seconds()
	} else if t.height == h.Height && h.Step == types.RoundStepPrevote {
		t.prevoteEnd = time.Now().Sub(t.prevoteStart).Seconds()
	} else if t.height == h.Height && h.Step == types.RoundStepPrecommit {
		t.precommitEnd = time.Now().Sub(t.precommitStart).Seconds()
	} else if t.height == h.Height && h.Step == types.RoundStepCommit {
		t.commitEnd = time.Now().Sub(t.commitStart).Seconds()
	}
}

func (t *TimeStatistic) AddStatistic(m map[string]int64) {
	if t.extraMapArr == nil {
		t.extraMapArr = make([]map[string]int64, 0)
	}
	t.extraMapArr = append(t.extraMapArr, m)
}

func (t *TimeStatistic) setStart() {
	t.startTime = time.Now().UnixNano() / 1e6
	//t.txNum = 0
}
func (t *TimeStatistic) setEnd() {
	t.endTime = time.Now().UnixNano() / 1e6
	t.totaltxNum = t.totaltxNum + t.txNum
}
func (t *TimeStatistic) SetEnterNewRoundStart() {
	t.enterNewRoundStart = time.Now().UnixNano() / 1e6
}

func (t *TimeStatistic) SetEnterProposalStart() {
	t.enterProposalStart = time.Now().UnixNano() / 1e6
}

func (t *TimeStatistic) SetEnterPreVoteStart() {
	t.enterPreVoteStart = time.Now().UnixNano() / 1e6
}

func (t *TimeStatistic) SetEnterPreCommitStart() {
	t.enterPreCommitStart = time.Now().UnixNano() / 1e6
}
func (t *TimeStatistic) SetChainID(chainID string) {
	t.chainID = chainID
}

func (t *TimeStatistic) SetHeight(height uint64) {
	t.height = height
}

func (t *TimeStatistic) SetRound(round int32) {
	t.round = round
}

func (t *TimeStatistic) SetTxNum(txNum int) {
	t.txNum = txNum
}

func (t *TimeStatistic) SetBeforeWriteBlock() {
	t.beforeWriteBlock = time.Now().UnixNano() / 1e6
}
func (t *TimeStatistic) SetAfterWriteBlock() {
	t.afterWriteBlock = time.Now().UnixNano() / 1e6
}
func (t *TimeStatistic) SetBeforeGetFrontOriginalBatch() {
	t.beforeGetFrontOriginalBatch = time.Now().UnixNano() / 1e6
}

func (t *TimeStatistic) SetAfterGetFrontOriginalBatch() {
	t.afterGetFrontOriginalBatch = time.Now().UnixNano() / 1e6
}

func NewTimeStatistic() *TimeStatistic {
	timeState := &TimeStatistic{}
	timeState.initTime = time.Now().UnixNano() / 1e6
	timeState.startTime = time.Now().UnixNano() / 1e6
	return timeState
}

func (t *TimeStatistic) String() string {
	return t.StringIndented("")
}

func (t *TimeStatistic) StringIndented(indent string) string {
	use := int(t.endTime-t.startTime) / 1000
	if use == 0 {
		use = 1
	}
	totalUse := int(t.endTime-t.initTime) / 1000
	if totalUse == 0 {
		totalUse = 1
	}
	totalTxNum := t.totaltxNum + t.txNum
	return fmt.Sprintf(`


	                  %v共识统计信息
%s  %v
-----------------------------------------------------------------------------
%s  BlockNum:%v Round:%v USE:%d(s) TPS:%v
%s  NewRound ->Propose              :                           %v ms
%s  Propose  ->PreVote              :                           %v ms
%s  PreVote  ->PreCommit            :                           %v ms
%s  PreCommit->End                  :                           %v ms
%s  WriteBlock                      :                           %v ms
%s  OriginalBatch                   :                           %v ms
-----------------------------------------------------------------------------
%s  TotalUse                        :                           %d s
%s  ToalTxNum                       :                           %d
%s  AverageTPS                      :                           %v
%s

	`,
		t.chainID,
		indent, time.Now().Format("2006-01-02 15:04:05"),
		indent, t.height, t.round, use, t.txNum/use,
		indent, t.enterProposalStart-t.enterNewRoundStart,
		indent, t.enterPreVoteStart-t.enterProposalStart,
		indent, t.enterPreCommitStart-t.enterPreVoteStart,
		indent, t.endTime-t.enterPreCommitStart,
		indent, t.afterWriteBlock-t.beforeWriteBlock,
		indent, t.afterGetFrontOriginalBatch-t.beforeGetFrontOriginalBatch,
		indent, totalUse,
		indent, totalTxNum,
		indent, totalTxNum/totalUse,
		indent)
}
