package prng_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/prng"
)

func TestResolveNextTimePoint(t *testing.T) {
	assert.EqualValues(t, 105, prng.ResolveNextTimePointSec(103, 5*time.Second))
	assert.EqualValues(t, 110, prng.ResolveNextTimePointSec(105, 5*time.Second))
	assert.EqualValues(t, 105, prng.ResolveNextTimePointSec(100, 5*time.Second))
	assert.EqualValues(t, 100, prng.ResolveNextTimePointSec(97, 5*time.Second))
}

func TestUnixTsPrng(t *testing.T) {
	unixTSRng := prng.NewUnixTimestampPRNG(1 * time.Second)
	unixTSRng.Start()
	defer unixTSRng.Stop()

	var last float64
	for i := 0; i < 3; i++ {
		r := <-unixTSRng.C()
		assert.Less(t, r, 1.0)
		assert.Greater(t, r, 0.0)
		assert.NotEqual(t, last, r)
		last = r
	}
}
