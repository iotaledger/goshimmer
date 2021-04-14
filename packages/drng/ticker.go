package drng

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
)

const (
	scaleFactor      = 100
	granularityCheck = 1000 / scaleFactor
)

// Ticker holds a channel that delivers randomness at intervals.
type Ticker struct {
	dRNGState    *State
	dRNGTicker   *time.Ticker
	resolution   int64 // the interval at which the ticker should tick (in seconds).
	defaultValue float64
	awaitOffset  int // defines the max amount of time (in seconds) to wait for the next dRNG round after the excected time has elapsed.
	c            chan float64
	exit         chan struct{}
	once         sync.Once
}

// NewTicker returns a pointer to a new Ticker.
func NewTicker(dRNGState *State, resolution int64, defaultValue float64, awaitOffset int) *Ticker {
	return &Ticker{
		dRNGState:    dRNGState,
		resolution:   resolution,
		defaultValue: defaultValue,
		awaitOffset:  awaitOffset,
		c:            make(chan float64),
		exit:         make(chan struct{}),
	}
}

// Start starts the Ticker.
func (t *Ticker) Start() {
	t.once.Do(func() {
		// TODO: improve this
		// Proposal-1: wait until you are in sync. FPC will start when the node is in sync
		// Proposal-2: use sync.once as soon as you solidfy a dRNG beacon (currently implemented).
		lastBeaconTime := t.dRNGState.Randomness().Timestamp.Unix()
		now := clock.SyncedTime().Unix()

		nextTimePoint := ResolveNextTimePoint(now, lastBeaconTime, t.resolution)
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
	})
}

// Stop stops the Ticker.
func (t *Ticker) Stop() {
	t.dRNGTicker.Stop()
	t.exit <- struct{}{}
}

// C returns the channel from which random generated numbers can be consumed from.
func (t *Ticker) C() <-chan float64 {
	return t.c
}

// sends the next random number to the consumer channel.
func (t *Ticker) send() {
	randomness := t.defaultValue
	for i := 0; i < t.awaitOffset*granularityCheck; i++ {
		if clock.Since(t.dRNGState.Randomness().Timestamp) < time.Duration(t.awaitOffset)*time.Second {
			randomness = t.dRNGState.Randomness().Float64()
			t.dRNGTicker.Reset(time.Duration(t.resolution) * time.Second)
			break
		}
		time.Sleep(scaleFactor * time.Millisecond)
	}

	// skip slow consumers
	select {
	case t.c <- randomness:
	default:
	}
}

// ResolveNextTimePoint returns the next time point.
// resolution - 10
// 19 - lastBeacon
// 45 - nowSec
// 49 - result
func ResolveNextTimePoint(nowSec, targetSec, resolution int64) int64 {
	return nowSec + resolution - (nowSec-targetSec)%resolution
}
