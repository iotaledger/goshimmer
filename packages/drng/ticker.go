package drng

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
)

const (
	scaleFactor      = 100
	granularityCheck = 1000 / scaleFactor
)

// Ticker holds a channel that delivers randomness at intervals.
type Ticker struct {
	dRNGState    func() *State
	dRNGTicker   *time.Ticker
	resolution   int64 // the interval at which the ticker should tick (in seconds).
	defaultValue float64
	awaitOffset  int // defines the max amount of time (in seconds) to wait for the next dRNG round after the excected time has elapsed.
	missingDRNG  bool
	c            chan float64
	exit         chan struct{}
}

// NewTicker returns a pointer to a new Ticker.
func NewTicker(dRNGState func() *State, resolution int64, defaultValue float64, awaitOffset int) *Ticker {
	return &Ticker{
		dRNGState:    dRNGState,
		resolution:   resolution,
		defaultValue: defaultValue,
		awaitOffset:  awaitOffset,
		missingDRNG:  true,
		c:            make(chan float64),
		exit:         make(chan struct{}),
	}
}

// Start starts the Ticker.
func (t *Ticker) Start() {
	now := clock.SyncedTime().Unix()
	nextTimePoint := ResolveNextTimePoint(now, t.resolution)
	time.AfterFunc(time.Duration(nextTimePoint-now)*time.Second, func() {
		// send for the first time right after the timer is executed
		t.send()

		t.dRNGTicker = time.NewTicker(time.Duration(t.resolution) * time.Second)
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

// sends the next random number to the consumer channel.
func (t *Ticker) send() {
	randomness := t.defaultValue
	if t.dRNGState() != nil {
		// wait for next randomness from dRNG
		for i := 0; i < t.awaitOffset*granularityCheck; i++ {
			if t.dRNGTicker != nil && t.missingDRNG && clock.Since(t.dRNGState().Randomness().Timestamp) < time.Duration(t.resolution)*time.Second {
				t.missingDRNG = false
				timeToNextDRNG := t.dRNGState().Randomness().Timestamp.Add(time.Duration(t.resolution) * time.Second).Sub(clock.SyncedTime())
				t.dRNGTicker.Reset(timeToNextDRNG)
			}
			if clock.Since(t.dRNGState().Randomness().Timestamp) < time.Duration(t.awaitOffset)*time.Second {
				randomness = t.dRNGState().Randomness().Float64()
				if t.dRNGTicker != nil {
					t.dRNGTicker.Reset(time.Duration(t.resolution) * time.Second)
				}
				break
			}
			time.Sleep(scaleFactor * time.Millisecond)
		}
	}

	// skip slow consumers
	select {
	case t.c <- randomness:
	default:
	}
}

// ResolveNextTimePoint returns the next time point.
func ResolveNextTimePoint(nowSec, resolution int64) int64 {
	return nowSec + resolution - nowSec%resolution
}
