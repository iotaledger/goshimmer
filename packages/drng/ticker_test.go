package drng

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func testRandomness(t time.Time) *Randomness {
	r := &Randomness{
		Round:      0,
		Randomness: make([]byte, 32),
		Timestamp:  t,
	}
	rand.Read(r.Randomness)
	return r
}

// Test that the
func TestTicker(t *testing.T) {
	testInterval := int64(10)
	randDefault := 0.6
	awaitOffset := int64(3)

	fmt.Println("=========== rand event arrives just before (interval) =========== ")
	randResult, randInput, delay := tickerFunc(time.Duration(testInterval-1) * time.Second)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(0)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("=========== rand event arrives just after (interval) =========== ")
	randResult, randInput, delay = tickerFunc(time.Duration(testInterval+1) * time.Second)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(0)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("=========== rand event arrives arrives just before (interval+awaitOffset) =========== ")
	randResult, randInput, delay = tickerFunc(time.Duration(testInterval+awaitOffset-1) * time.Second)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(0)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("=========== rand event arrives arrives just after (interval+awaitOffset) =========== ")
	randResult, randInput, delay = tickerFunc(time.Duration(testInterval+awaitOffset+1) * time.Second)
	assert.Equal(t, randResult, randDefault)
	require.InDelta(t, time.Duration(awaitOffset)*time.Second, delay, float64(100*time.Millisecond))

}

func tickerFunc(timestamp time.Duration) (randResult, randInput float64, delay time.Duration) {

	testInterval := int64(10)
	randDefault := 0.6
	awaitOffset := int64(3)
	var testState *State
	testState = NewState(SetCommittee(dummyCommittee()), SetRandomness(testRandomness(time.Now())))
	stateFunc := func() *State { return testState }
	ticker := NewTicker(stateFunc, testInterval, randDefault, awaitOffset)

	ticker.testStart()
	defer ticker.Stop()
	start := time.Now()
	timestampedRandomness := testRandomness(time.Now().Add(timestamp))
	randInput = timestampedRandomness.Float64()
	fmt.Println("tickerFunc_______  Timestamp, randTimestamp :: ", clock.Since(timestampedRandomness.Timestamp), ", ", randInput, " _______", time.Since(start), "(note, neg timestamp is in the future)")
	// mock the dRNG event
	go func() {
		time.Sleep(timestamp)
		ticker.dRNGState().UpdateRandomness(timestampedRandomness)
		ticker.UpdateRandomness(*timestampedRandomness)
	}()

	// listen to the randomness event after some delay
	go func() {
		time.Sleep(5 * time.Second)
		randResult = <-ticker.C()
	}()

	ticker.missingDRNG = true
	// sleep till everything is done
	time.Sleep(time.Duration(testInterval)*time.Second + 5*time.Second)
	delay = ticker.DelayedRoundStart()

	return
}

func TestNoDRNGTicker(t *testing.T) {
	interval := int64(5)
	defaultValue := 0.6
	awaitOffset := int64(3)
	stateFunc := func() *State { return nil }

	ticker := NewTicker(stateFunc, interval, defaultValue, awaitOffset)

	ticker.Start()
	defer ticker.Stop()

	r := <-ticker.C()
	assert.Equal(t, r, defaultValue)

	r = <-ticker.C()
	assert.Equal(t, r, defaultValue)
}

// Start starts the Ticker.
func (t *Ticker) testStart() {
	time.AfterFunc(0, func() {
		t.dRNGTicker = time.NewTicker(time.Duration(t.interval) * time.Second)
		// send for the first time right after the timer is started
		t.send()
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
