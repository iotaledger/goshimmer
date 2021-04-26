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

func TestSendIndividually(t *testing.T) {
	testInterval := int64(10)
	randDefault := 0.6
	awaitOffset := int64(3)

	fmt.Println("+++++++++++ in sync and out of sync with dRNG , dRNG not missing +++++++++++ ")

	timestamp := time.Duration(testInterval-4) * time.Second
	randResult, randInput, delay := tickerFunc(timestamp, timestamp, false)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(4)*time.Second, delay, float64(100*time.Millisecond))

	timestamp = time.Duration(testInterval-4) * time.Second
	randResult, randInput, delay = tickerFunc(timestamp, timestamp+2*time.Second, false)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(4)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("+++++++++++ in sync with dRNG , dRNG missing +++++++++++ ")

	fmt.Println("=========== rand event arrives before (interval)  =========== ")
	timestamp = time.Duration(testInterval-4) * time.Second
	randResult, randInput, delay = tickerFunc(timestamp, timestamp, true)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(4)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("=========== rand event arrives just after (interval) =========== ")
	timestamp = time.Duration(testInterval+1) * time.Second
	randResult, randInput, delay = tickerFunc(timestamp, timestamp, true)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(0)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("=========== rand event arrives just before (interval+awaitOffset) =========== ")
	timestamp = time.Duration(testInterval+awaitOffset-1) * time.Second
	randResult, randInput, delay = tickerFunc(timestamp, timestamp, true)
	assert.Equal(t, randResult, randInput)
	require.InDelta(t, time.Duration(0)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("=========== rand event arrives just after (interval+awaitOffset) =========== ")
	timestamp = time.Duration(testInterval+awaitOffset+1) * time.Second
	randResult, _, delay = tickerFunc(timestamp, timestamp, true)
	assert.Equal(t, randResult, randDefault)
	require.InDelta(t, time.Duration(awaitOffset)*time.Second, delay, float64(100*time.Millisecond))

	fmt.Println("+++++++++++ out of sync with dRNG , dRNG missing +++++++++++ ")

	fmt.Println("=========== rand event arrives before (interval) but timestamp out of sync =========== ")
	timestamp = time.Duration(testInterval-2) * time.Second
	randResult, randInput, delay = tickerFunc(timestamp, timestamp+4*time.Second, true)
	assert.Equal(t, randResult, randDefault)
	require.InDelta(t, time.Duration(3)*time.Second, delay, float64(100*time.Millisecond))

	timestamp = time.Duration(testInterval-2) * time.Second
	randResult, _, delay = tickerFunc(timestamp, timestamp+6*time.Second, true)
	assert.Equal(t, randResult, randDefault)
	require.InDelta(t, time.Duration(awaitOffset)*time.Second, delay, float64(100*time.Millisecond))
}

func tickerFunc(timestamp, timestampSendTime time.Duration, missingDRNG bool) (randResult, randInput float64, delay time.Duration) {
	testInterval := int64(10)
	randDefault := 0.6
	awaitOffset := int64(3)
	testState := NewState(SetCommittee(dummyCommittee()), SetRandomness(testRandomness(time.Now())))
	stateFunc := func() *State { return testState }
	ticker := NewTicker(stateFunc, testInterval, randDefault, awaitOffset)

	ticker.Start()
	defer ticker.Stop()

	ticker.missingDRNG = missingDRNG

	<-ticker.C()
	timestampedRandomness := testRandomness(time.Now().Add(timestamp))
	randInput = timestampedRandomness.Float64()

	fmt.Println("tickerFunc_______  Timestamp, timestampSendTime, randInput :: ", timestamp, timestampSendTime, randInput)

	// mock the dRNG event
	go func() {
		time.Sleep(timestampSendTime)
		ticker.dRNGState().UpdateRandomness(timestampedRandomness)
		ticker.UpdateRandomness(*timestampedRandomness)
	}()

	randResult = <-ticker.C()
	delay = ticker.DelayedRoundStart()

	return randResult, randInput, delay
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
