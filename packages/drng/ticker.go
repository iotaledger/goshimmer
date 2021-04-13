package drng

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
)

const (
	scaleFactor = 100
)

// MaxdRNGAwait defines the max amount of time (in seconds) to wait for the next
// dRNG round after the excected time has elapsed.
var MaxdRNGAwait = 3

// Ticker holds a channel that delivers randomness at intervals.
type Ticker struct {
	dRNGState    *State
	dRNGTicker   *time.Ticker
	resolution   int64
	defaultValue float64
	c            chan float64
	exit         chan struct{}
}

// NewTicker returns a pointer to a new Ticker.
func NewTicker(dRNGState *State, resolution int64, defaultValue float64) *Ticker {
	return &Ticker{
		dRNGState:    dRNGState,
		resolution:   resolution,
		defaultValue: defaultValue,
		c:            make(chan float64),
		exit:         make(chan struct{}),
	}
}

// Start starts the Ticker.
func (t *Ticker) Start() {
	// TODO: improve this
	lastBeaconTime := t.dRNGState.Randomness().Timestamp.Unix()

	nextTimePoint := ResolveNextTimePoint(lastBeaconTime, t.resolution)
	time.AfterFunc(time.Duration(nextTimePoint-lastBeaconTime)*time.Second, func() {
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
	for i := 0; i < MaxdRNGAwait*10; i++ {
		if clock.Since(t.dRNGState.Randomness().Timestamp) < time.Duration(MaxdRNGAwait)*time.Second {
			randomness = t.dRNGState.Randomness().Float64()
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
func ResolveNextTimePoint(nowSec, resolution int64) int64 {
	return nowSec + (resolution - nowSec%resolution)
}
