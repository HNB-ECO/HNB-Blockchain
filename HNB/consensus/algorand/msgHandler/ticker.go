package msgHandler

import (
	cmn "github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/common"
	"github.com/HNB-ECO/HNB-Blockchain/HNB/consensus/algorand/types"
	"time"
)

var (
	tickTockBufferSize = 10
)

// internally generated messages which may update the state
type timeoutInfo struct {
	Duration time.Duration       `json:"duration"`
	Height   uint64              `json:"height"`
	Round    int32               `json:"round"`
	Step     types.RoundStepType `json:"step"`
}

// timeoutTicker is a timer that schedules timeouts
// conditional on the height/round/step in the timeoutInfo.
// The timeoutInfo.Duration may be non-positive.
type TimeoutTicker interface {
	Start() error
	Stop() error
	Reset() error
	OnReset() error
	Chan() <-chan timeoutInfo       // on which to receive a timeout
	ScheduleTimeout(ti timeoutInfo) // reset the timer
}

// timeoutTicker wraps time.Timer,
// scheduling timeouts only for greater height/round/step
// than what it's already seen.
// Timeouts are scheduled along the tickChan,
// and fired on the tockChan.
type timeoutTicker struct {
	cmn.BaseService

	timer    *time.Timer
	tickChan chan timeoutInfo // for scheduling timeouts
	tockChan chan timeoutInfo // for notifying about them
}

// NewTimeoutTicker returns a new timeoutTicker.
func NewTimeoutTicker() TimeoutTicker {
	tt := &timeoutTicker{
		timer:    time.NewTimer(0),
		tickChan: make(chan timeoutInfo, tickTockBufferSize),
		tockChan: make(chan timeoutInfo, tickTockBufferSize),
	}
	tt.BaseService = *cmn.NewBaseService("timeoutTicker", tt)
	tt.stopTimer() // don't want to fire until the first scheduled timeout
	return tt
}

// OnStart implements Service. It starts the timeout routine.
func (t *timeoutTicker) OnStart() error {

	go t.timeoutRoutine()

	return nil
}

// OnStop implements Service. It stops the timeout routine.
func (t *timeoutTicker) OnStop() {
	t.BaseService.OnStop()
	t.stopTimer()
}

// Chan returns a channel on which timeouts are sent.
func (t *timeoutTicker) Chan() <-chan timeoutInfo {
	return t.tockChan
}

// ScheduleTimeout schedules a new timeout by sending on the internal tickChan.
// The timeoutRoutine is always available to read from tickChan, so this won't block.
// The scheduling may fail if the timeoutRoutine has already scheduled a timeout for a later height/round/step.
func (t *timeoutTicker) ScheduleTimeout(ti timeoutInfo) {
	t.tickChan <- ti
}

//-------------------------------------------------------------

// stop the timer and drain if necessary
func (t *timeoutTicker) stopTimer() {
	// Stop() returns false if it was already fired or was stopped
	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
			//t.Logger.Debug("Timer already stopped")
		}
	}
}

// send on tickChan to start a new timer.
// timers are interupted and replaced by new ticks from later steps
// timeouts of 0 on the tickChan will be immediately relayed to the tockChan
func (t *timeoutTicker) timeoutRoutine() {
	var ti timeoutInfo
	for {
		select {
		case newti := <-t.tickChan:
			//	t.Logger.Debug("Received tick", "old_ti", ti, "new_ti", newti)

			// ignore tickers for old height/round/step
			if newti.Height < ti.Height {
				continue
			} else if newti.Height == ti.Height {
				if newti.Round < ti.Round {
					continue
				} else if newti.Round == ti.Round {
					if ti.Step > 0 && newti.Step < ti.Step {
						continue
					}
				}
			}

			// stop the last timer
			t.stopTimer()

			// update timeoutInfo and reset timer
			// NOTE time.Timer allows duration to be non-positive
			ti = newti
			t.timer.Reset(ti.Duration)
			//t.Logger.Debug("Scheduled timeout", "dur", ti.Duration, "height", ti.BlockNum, "round", ti.Round, "step", ti.Step)
		case <-t.timer.C:
			//t.Logger.Info("Timed out", "dur", ti.Duration, "height", ti.BlockNum, "round", ti.Round, "step", ti.Step)
			// go routine here guarantees timeoutRoutine doesn't block.
			// Determinism comes from playback in the receiveRoutine.
			// We can eliminate it by merging the timeoutRoutine into receiveRoutine
			//  and managing the timeouts ourselves with a millisecond ticker
			go func(toi timeoutInfo) { t.tockChan <- toi }(ti)
		case <-t.Quit():
			return
		}
	}
}

func (t *timeoutTicker) OnReset() error {
	t.tickChan = make(chan timeoutInfo, tickTockBufferSize)
	t.tockChan = make(chan timeoutInfo, tickTockBufferSize)

	return nil
}
