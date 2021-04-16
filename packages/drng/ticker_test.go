package drng

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
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

func TestTicker(t *testing.T) {
	interval := 5
	defaultValue := 0.6
	awaitOffset := 3
	var stateTest *State
	stateFunc := func() *State { return stateTest }

	ticker := NewTicker(stateFunc, int64(interval), defaultValue, awaitOffset)

	ticker.Start()
	defer ticker.Stop()

	r := <-ticker.C()
	assert.Equal(t, r, defaultValue)

	stateTest = NewState(SetCommittee(dummyCommittee()), SetRandomness(testRandomness(time.Now().Add(time.Duration(interval)*time.Second))))
	randomness := stateTest.Randomness().Float64()
	r = <-ticker.C()
	assert.Equal(t, r, randomness)
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
