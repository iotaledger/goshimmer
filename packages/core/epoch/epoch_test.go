package epoch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEpoch(t *testing.T) {
	genesisTime := time.Unix(GenesisTime, 0)

	{
		endOfEpochTime := genesisTime.Add(time.Duration(Duration) * time.Second).Add(-1)

		assert.Equal(t, Index(1), IndexFromTime(endOfEpochTime))
		assert.False(t, Index(1).EndTime().Before(endOfEpochTime))

		startOfEpochTime := genesisTime.Add(time.Duration(Duration) * time.Second)

		assert.Equal(t, Index(2), IndexFromTime(startOfEpochTime))
		assert.False(t, Index(2).StartTime().After(startOfEpochTime))
	}

	{
		testTime := genesisTime.Add(5 * time.Second)
		index := IndexFromTime(testTime)
		assert.Equal(t, index, Index(1))

		startTime := index.StartTime()
		assert.Equal(t, startTime, time.Unix(genesisTime.Unix(), 0))
		endTime := index.EndTime()
		assert.Equal(t, endTime, (index + 1).StartTime().Add(-1))
	}

	{
		testTime := genesisTime.Add(10 * time.Second)
		index := IndexFromTime(testTime)
		assert.Equal(t, index, Index(2))

		startTime := index.StartTime()
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(10*time.Second).Unix(), 0))
		endTime := index.EndTime()
		assert.Equal(t, endTime, (index + 1).StartTime().Add(-1))
	}

	{
		testTime := genesisTime.Add(35 * time.Second)
		index := IndexFromTime(testTime)
		assert.Equal(t, index, Index(4))

		startTime := index.StartTime()
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(30*time.Second).Unix(), 0))
		endTime := index.EndTime()
		assert.Equal(t, endTime, (index + 1).StartTime().Add(-1))
	}

	{
		testTime := genesisTime.Add(49 * time.Second)
		index := IndexFromTime(testTime)
		assert.Equal(t, index, Index(5))
	}

	{
		// a time before genesis time, index = 0
		testTime := genesisTime.Add(-10 * time.Second)
		index := IndexFromTime(testTime)
		assert.Equal(t, index, Index(0))
	}
}
