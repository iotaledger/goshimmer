package drng

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
)

// Ticker holds a channel that delivers randomness at intervals.
type Ticker struct {
	dRNGState           func() *State
	dRNGTicker          *time.Ticker
	interval            int64 // the interval at which the ticker should tick (in seconds).
	defaultValue        float64
	awaitOffset         int64 // defines the max amount of time (in seconds) to wait for the next dRNG round after the expected time has elapsed.
	missingDRNG         bool
	delayedRoundStart   time.Duration
	delayedRoundStartMu sync.RWMutex
	c                   chan float64
	exit                chan struct{}
	fromRandomnessEvent chan Randomness
}

// NewTicker returns a pointer to a new Ticker.
func NewTicker(dRNGState func() *State, interval int64, defaultValue float64, awaitOffset int64) *Ticker {
	return &Ticker{
		dRNGState:           dRNGState,
		interval:            interval,
		defaultValue:        defaultValue,
		awaitOffset:         awaitOffset,
		missingDRNG:         true,
		c:                   make(chan float64),
		exit:                make(chan struct{}),
		fromRandomnessEvent: make(chan Randomness),
	}
}

// UpdateRandomness updates the randomness of the ticker.
func (t *Ticker) UpdateRandomness(r Randomness) {
	select {
	case <-t.fromRandomnessEvent:
		t.fromRandomnessEvent <- r
	default:
		t.fromRandomnessEvent <- r
	}
}

// Start starts the Ticker.
func (t *Ticker) Start() {
	now := clock.SyncedTime().Unix()
	nextTimePoint := ResolveNextTimePoint(now, t.interval)
	time.AfterFunc(time.Duration(nextTimePoint-now)*time.Second, func() {
		// send for the first time right after the timer is executed
		t.send()

		t.dRNGTicker = time.NewTicker(time.Duration(t.interval) * time.Second)
		defer t.Stop()
	out:
		for {
			select {
			case <-t.dRNGTicker.C:
				t.send()
			case <-t.exit:
				break out
			}
		}
	})
}

// Stop stops the Ticker.
func (t *Ticker) Stop() {
	t.exit <- struct{}{}
	t.dRNGTicker.Stop()
}

// C returns the channel from which random generated numbers can be consumed from.
func (t *Ticker) C() <-chan float64 {
	return t.c
}

// DelayedRoundStart returns how much the current Round is delayed already.
func (t *Ticker) DelayedRoundStart() time.Duration {
	t.delayedRoundStartMu.RLock()
	defer t.delayedRoundStartMu.RUnlock()
	return t.delayedRoundStart
}

func (t *Ticker) setDelayedRoundStart(d time.Duration) {
	t.delayedRoundStartMu.Lock()
	defer t.delayedRoundStartMu.Unlock()
	t.delayedRoundStart = d
}

// sends the next random number to the consumer channel.
func (t *Ticker) send() {
	t.setDelayedRoundStart(0)
	randomness := t.defaultValue

	if t.dRNGState() != nil && t.dRNGTicker != nil {
		// check if the randomness is "fresh"
		if t.missingDRNG && clock.Since(t.dRNGState().Randomness().Timestamp) < time.Duration(t.interval)*time.Second {
			t.missingDRNG = false
			randomness = t.dRNGState().Randomness().Float64()
			// the expected time that we should receive a new randomness
			timeToNextDRNG := t.dRNGState().Randomness().Timestamp.Add(time.Duration(t.interval) * time.Second).Sub(clock.SyncedTime())
			t.dRNGTicker.Reset(timeToNextDRNG)
		} else {
			// set ticker to awaitOffset
			t.dRNGTicker.Reset(time.Duration(t.awaitOffset) * time.Second)
		}
	out:
		// if fresh randomness is not received, we wait for new randomness for awaitOffset seconds
		for {
			// abort if we already get the latest randomness
			if randomness != t.defaultValue {
				break out
			}
			select {
			// receive randomness from Randomness event
			case randomnessEvent := <-t.fromRandomnessEvent:
				// check if the randomness is "fresh"
				if t.dRNGTicker != nil {
					if clock.Since(randomnessEvent.Timestamp) < time.Duration(t.awaitOffset)*time.Second {
						randomness = t.dRNGState().Randomness().Float64()
						timeToNextDRNG := randomnessEvent.Timestamp.Add(time.Duration(t.interval) * time.Second).Sub(clock.SyncedTime())
						t.dRNGTicker.Reset(timeToNextDRNG)
						t.setDelayedRoundStart(time.Duration(t.interval)*time.Second - timeToNextDRNG)
					}
					break out
				}
			case <-t.dRNGTicker.C:
				// still no new randomness within awaitOffset, take the default value, and reset dRNGTicker
				t.setDelayedRoundStart(time.Duration(t.awaitOffset) * time.Second)
				t.dRNGTicker.Reset(time.Duration(time.Duration(t.interval-t.awaitOffset) * time.Second))
				// t.dRNGTicker.Reset(time.Duration(ResolveNextTimePoint(now, t.interval)-now) * time.Second)
				break out
			}
		}
	}

	// skip slow consumers
	select {
	case t.c <- randomness:
	default:
	}
}

// ResolveNextTimePoint returns the next time point.
func ResolveNextTimePoint(nowSec, interval int64) int64 {
	return nowSec + interval - nowSec%interval
}
