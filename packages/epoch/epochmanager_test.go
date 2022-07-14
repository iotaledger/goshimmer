package epoch

import (
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
)

func TestEpochManager(t *testing.T) {
	genesisTime := time.Now()
	GenesisTime = genesisTime.Unix()
	Duration = 10

	{
		// ei = 0
		testTime := genesisTime.Add(5 * time.Second)
		ei := IndexFromTime(testTime)
		assert.Equal(t, ei, Index(1))

		startTime := ei.StartTime()
		assert.Equal(t, startTime, time.Unix(genesisTime.Unix(), 0))
		endTime := ei.EndTime()
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(9*time.Second).Unix(), 0))
	}

	{
		// ei = 1
		testTime := genesisTime.Add(10 * time.Second)
		ei := IndexFromTime(testTime)
		assert.Equal(t, ei, Index(2))

		startTime := ei.StartTime()
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(10*time.Second).Unix(), 0))
		endTime := ei.EndTime()
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(19*time.Second).Unix(), 0))
	}

	{
		// ei = 3
		testTime := genesisTime.Add(35 * time.Second)
		ei := IndexFromTime(testTime)
		assert.Equal(t, ei, Index(4))

		startTime := ei.StartTime()
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(30*time.Second).Unix(), 0))
		endTime := ei.EndTime()
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(39*time.Second).Unix(), 0))
	}

	{
		// ei = 4
		testTime := genesisTime.Add(49 * time.Second)
		ei := IndexFromTime(testTime)
		assert.Equal(t, ei, Index(5))

	}

	{
		// a time before genesis time, ei = 0
		testTime := genesisTime.Add(-10 * time.Second)
		ei := IndexFromTime(testTime)
		assert.Equal(t, ei, Index(0))
	}
}
