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
	interval            time.Duration // the interval at which the ticker should tick.
	defaultValue        float64
	awaitOffset         time.Duration // defines the max amount of time to wait for the next dRNG round after the expected time has elapsed.
	missingDRNG         bool
	delayedRoundStart   time.Duration
	delayedRoundStartMu sync.RWMutex
	c                   chan float64
	exit                chan struct{}
	fromRandomnessEvent chan Randomness
}

// NewTicker returns a pointer to a new Ticker.
func NewTicker(dRNGState func() *State, interval time.Duration, defaultValue float64, awaitOffset time.Duration) *Ticker {
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
	for len(t.fromRandomnessEvent) > 0 {
		<-t.fromRandomnessEvent
	}
	select {
	case t.fromRandomnessEvent <- r:
	default:
	}
}

// Start starts the Ticker.
func (t *Ticker) Start() {
	nowSec := clock.SyncedTime().Unix()
	nextTimePoint := ResolveNextTimePoint(time.Duration(nowSec)*time.Second, t.interval)
	time.AfterFunc(nextTimePoint-time.Duration(nowSec)*time.Second, func() {
		// send for the first time right after the timer is executed
		t.send()

		t.dRNGTicker = time.NewTicker(t.interval)
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
	now := clock.SyncedTime()

	if t.dRNGState() != nil && t.dRNGTicker != nil {
		// we expect a randomness, it arrived already, and its newer than the last one
		// OR we were missing a dRNG message previously but the check if the last randomness received is "fresh"
		if now.Sub(t.dRNGState().Randomness().Timestamp) < t.interval && now.Sub(t.dRNGState().Randomness().Timestamp) > 0 {
			t.missingDRNG = false
			randomness = t.dRNGState().Randomness().Float64()
			timeToNextDRNG := t.dRNGState().Randomness().Timestamp.Add(t.interval).Sub(now)
			t.setDelayedRoundStart(t.interval - timeToNextDRNG)
			t.dRNGTicker.Reset(timeToNextDRNG)
		} else {
			// set ticker to awaitOffset, to await arrival of randomness event, continue at out:
			t.dRNGTicker.Reset(t.awaitOffset)
		}
	out:
		// if fresh randomness is not received, we wait for new randomness for awaitOffset seconds
		for {
			// abort if we already got the latest randomness
			if randomness != t.defaultValue {
				break out
			}
			select {
			// receive or consume randomness from Randomness event
			case randomnessEvent := <-t.fromRandomnessEvent:
				// check if the randomness is "fresh"
				if t.dRNGTicker != nil {
					now := clock.SyncedTime()
					// check against awaitOffset to be no more than 6 seconds
					if now.Sub(randomnessEvent.Timestamp) < t.awaitOffset && now.Sub(randomnessEvent.Timestamp) > 0 {
						t.missingDRNG = false
						randomness = t.dRNGState().Randomness().Float64()
						timeToNextDRNG := randomnessEvent.Timestamp.Add(t.interval).Sub(now)
						t.dRNGTicker.Reset(timeToNextDRNG)
						t.setDelayedRoundStart(t.interval - timeToNextDRNG)
						break out
					}
				}
			case <-t.dRNGTicker.C:
				t.missingDRNG = true
				// still no new randomness within awaitOffset, take the default value, and reset dRNGTicker
				t.setDelayedRoundStart(t.awaitOffset)
				t.dRNGTicker.Reset(t.interval - t.awaitOffset)
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
func ResolveNextTimePoint(nowTime, interval time.Duration) time.Duration {
	return nowTime + interval - nowTime%interval
}
