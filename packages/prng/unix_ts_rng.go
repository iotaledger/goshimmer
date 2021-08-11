package prng

import (
	"math/rand"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
)

// TimeSourceFunc is a function which gets an understanding of time in seconds resolution back.
type TimeSourceFunc func() int64

// NewUnixTimestampPRNG creates a new Unix timestamp based pseudo random number generator
// using the given interval. The interval defines at which second interval numbers are generated.
func NewUnixTimestampPRNG(interval time.Duration, timeSourceFunc ...TimeSourceFunc) *UnixTimestampPrng {
	utrng := &UnixTimestampPrng{
		c:              make(chan float64),
		exit:           make(chan struct{}),
		interval:       interval,
		timeSourceFunc: func() int64 { return clock.SyncedTime().Unix() },
	}
	if len(timeSourceFunc) > 0 {
		utrng.timeSourceFunc = timeSourceFunc[0]
	}
	return utrng
}

// UnixTimestampPrng is a pseudo random number generator using the Unix time in seconds to derive
// a random number from.
type UnixTimestampPrng struct {
	c              chan float64
	exit           chan struct{}
	interval       time.Duration
	timeSourceFunc TimeSourceFunc
}

// Start starts the Unix timestamp pseudo random number generator by examining the
// interval and then starting production of numbers after at least interval seconds
// plus delta of the next interval time have elapsed.
func (utrng *UnixTimestampPrng) Start() {
	nowSec := utrng.timeSourceFunc()
	nextTimePointSec := ResolveNextTimePointSec(nowSec, utrng.interval)
	time.AfterFunc(time.Duration(nextTimePointSec-nowSec)*time.Second, func() {
		// send for the first time right after the timer is executed
		utrng.send()

		t := time.NewTicker(utrng.interval)
		defer t.Stop()
	out:
		for {
			select {
			case <-t.C:
				utrng.send()
			case <-utrng.exit:
				break out
			}
		}
	})
}

// sends the next pseudo random number to the consumer channel.
func (utrng *UnixTimestampPrng) send() {
	nowSec := utrng.timeSourceFunc()
	// reduce to last interval
	timePointSec := nowSec - (nowSec % int64(utrng.interval/time.Second))

	// add entropy and convert to float64
	pseudoR := rand.New(rand.NewSource(timePointSec)).Float64()

	// skip slow consumers
	select {
	case utrng.c <- pseudoR:
	default:
	}
}

// C returns the channel from which random generated numbers can be consumed from.
func (utrng *UnixTimestampPrng) C() <-chan float64 {
	return utrng.c
}

// Stop stops the Unix timestamp pseudo random number generator.
func (utrng *UnixTimestampPrng) Stop() {
	utrng.exit <- struct{}{}
}

// ResolveNextTimePointSec returns the next time point.
func ResolveNextTimePointSec(nowSec int64, interval time.Duration) int64 {
	intervalSec := int64(interval / time.Second)
	return nowSec + (intervalSec - nowSec%intervalSec)
}
