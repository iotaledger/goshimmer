package epoch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEpoch(t *testing.T) {
	timeProvider := NewTimeProvider()
	genesisTime := timeProvider.GenesisTime()

	{
		endOfEpochTime := genesisTime.Add(time.Duration(timeProvider.Duration()) * time.Second).Add(-1)

		assert.Equal(t, Index(1), timeProvider.IndexFromTime(endOfEpochTime))
		assert.False(t, timeProvider.EndTime(Index(1)).Before(endOfEpochTime))

		startOfEpochTime := genesisTime.Add(time.Duration(timeProvider.Duration()) * time.Second)

		assert.Equal(t, Index(2), timeProvider.IndexFromTime(startOfEpochTime))
		assert.False(t, timeProvider.StartTime(Index(2)).After(startOfEpochTime))
	}

	{
		testTime := genesisTime.Add(5 * time.Second)
		index := timeProvider.IndexFromTime(testTime)
		assert.Equal(t, index, Index(1))

		startTime := timeProvider.StartTime(index)
		assert.Equal(t, startTime, time.Unix(genesisTime.Unix(), 0))
		endTime := timeProvider.EndTime(index)
		assert.Equal(t, endTime, timeProvider.StartTime(index+1).Add(-1))
	}

	{
		testTime := genesisTime.Add(10 * time.Second)
		index := timeProvider.IndexFromTime(testTime)
		assert.Equal(t, index, Index(2))

		startTime := timeProvider.StartTime(index)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(10*time.Second).Unix(), 0))
		endTime := timeProvider.EndTime(index)
		assert.Equal(t, endTime, timeProvider.StartTime(index+1).Add(-1))
	}

	{
		testTime := genesisTime.Add(35 * time.Second)
		index := timeProvider.IndexFromTime(testTime)
		assert.Equal(t, index, Index(4))

		startTime := timeProvider.StartTime(index)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(30*time.Second).Unix(), 0))
		endTime := timeProvider.EndTime(index)
		assert.Equal(t, endTime, timeProvider.StartTime(index+1).Add(-1))
	}

	{
		testTime := genesisTime.Add(49 * time.Second)
		index := timeProvider.IndexFromTime(testTime)
		assert.Equal(t, index, Index(5))
	}

	{
		// a time before genesis time, index = 0
		testTime := genesisTime.Add(-10 * time.Second)
		index := timeProvider.IndexFromTime(testTime)
		assert.Equal(t, index, Index(0))
	}
}
