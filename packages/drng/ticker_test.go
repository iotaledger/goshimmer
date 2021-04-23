package drng

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

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
	interval := 5
	defaultValue := 0.6
	awaitOffset := 3
	var stateTest *State
	stateFunc := func() *State { return stateTest }

	ticker := NewTicker(stateFunc, int64(interval), defaultValue, awaitOffset)

	ticker.Start()
	defer ticker.Stop()

	// no dRNG event
	fmt.Println("=========== no dRNG event =========== ")
	r := <-ticker.C()
	assert.Equal(t, r, defaultValue)
	fmt.Println(ticker.DelayedRoundStart())

	// event arrives before (interval+awaitOffset)
	fmt.Println("=========== event arrives before (interval+awaitOffset) =========== ")
	timestamp := time.Duration(interval) * time.Second
	stateTest = NewState(SetCommittee(dummyCommittee()), SetRandomness(testRandomness(time.Now().Add(timestamp))))
	randomness := stateTest.Randomness().Float64()
	fmt.Println(stateTest.randomness.Timestamp, randomness)
	// mock the dRNG event
	go func() {
		time.Sleep(timestamp)
		ticker.UpdateRandomness(stateTest.Randomness())
	}()
	r = <-ticker.C()
	fmt.Println("r= ", r, ", randomness=", randomness)
	assert.Equal(t, r, randomness)
	fmt.Println(ticker.DelayedRoundStart())

	// event arrives after (interval+awaitOffset)
	fmt.Println("=========== event arrives after (interval+awaitOffset) =========== ")
	timestamp = time.Duration(interval+awaitOffset+1) * time.Second
	stateTest = NewState(SetCommittee(dummyCommittee()), SetRandomness(testRandomness(time.Now().Add(timestamp))))
	randomness = stateTest.Randomness().Float64()
	fmt.Println(stateTest.randomness.Timestamp, randomness, time.Now())
	// mock the dRNG event
	go func() {
		time.Sleep(timestamp)
		fmt.Println("before UpdateRandomness", time.Now())
		ticker.UpdateRandomness(stateTest.Randomness())
		fmt.Println("after UpdateRandomness", time.Now())
	}()
	fmt.Println("....... ticker not yet ticked", time.Now())
	r = <-ticker.C()
	fmt.Println("....... ticker ticked", time.Now())
	fmt.Println("r= ", r, ", randomness =", randomness, time.Now())
	assert.Equal(t, r, defaultValue)
	require.InDelta(t, time.Duration(awaitOffset)*time.Second, ticker.DelayedRoundStart(), float64(100*time.Millisecond))
	fmt.Println(ticker.DelayedRoundStart())

	// event arrives after (2*interval)
	fmt.Println("=========== event arrives after (2*interval) =========== ")
	timestamp = time.Duration(2*interval+1) * time.Second
	stateTest = NewState(SetCommittee(dummyCommittee()), SetRandomness(testRandomness(time.Now().Add(timestamp))))
	randomness = stateTest.Randomness().Float64()
	// mock the dRNG event
	go func() {
		time.Sleep(timestamp)
		ticker.UpdateRandomness(stateTest.Randomness())
	}()
	r = <-ticker.C()
	fmt.Println("r= ", r, ", randomness=", randomness)
	assert.Equal(t, r, randomness)
	require.InDelta(t, 1, ticker.DelayedRoundStart(), float64(100*time.Millisecond))
	fmt.Println(ticker.DelayedRoundStart())

}

func TestNoDRNGTicker(t *testing.T) {
	interval := 5
	defaultValue := 0.6
	awaitOffset := 3
	stateFunc := func() *State { return nil }

	ticker := NewTicker(stateFunc, int64(interval), defaultValue, awaitOffset)

	ticker.Start()
	defer ticker.Stop()

	r := <-ticker.C()
	assert.Equal(t, r, defaultValue)

	r = <-ticker.C()
	assert.Equal(t, r, defaultValue)
}
