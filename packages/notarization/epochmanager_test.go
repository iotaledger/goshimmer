package notarization

import (
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
)

func TestEpochManager(t *testing.T) {
	genesisTime := time.Now()
	manager := NewEpochManager(GenesisTime(genesisTime.Unix()), Interval(int64(10)))

	{
		// ei = 0
		testTime := genesisTime.Add(5 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, EI(0))

		oracleEci := manager.TimeToOracleEI(testTime)
		assert.Equal(t, oracleEci, EI(0))

		startTime := manager.EIToStartTime(ei)
		assert.Equal(t, startTime, time.Unix(genesisTime.Unix(), 0))
		endTime := manager.EIToEndTime(ei)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(9*time.Second).Unix(), 0))
	}

	{
		// ei = 1
		testTime := genesisTime.Add(10 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, EI(1))

		oracleEci := manager.TimeToOracleEI(testTime)
		assert.Equal(t, oracleEci, EI(0))

		startTime := manager.EIToStartTime(ei)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(10*time.Second).Unix(), 0))
		endTime := manager.EIToEndTime(ei)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(19*time.Second).Unix(), 0))
	}

	{
		// ei = 3
		testTime := genesisTime.Add(35 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, EI(3))

		oracleEci := manager.TimeToOracleEI(testTime)
		assert.Equal(t, oracleEci, EI(0))

		startTime := manager.EIToStartTime(ei)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(30*time.Second).Unix(), 0))
		endTime := manager.EIToEndTime(ei)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(39*time.Second).Unix(), 0))
	}

	{
		// ei = 4
		testTime := genesisTime.Add(49 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, EI(4))

		oracleEci := manager.TimeToOracleEI(testTime)
		assert.Equal(t, oracleEci, EI(2))
	}

	{
		// a time before genesis time, ei = 0
		testTime := genesisTime.Add(-10 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, EI(0))

		oracleEci := manager.TimeToOracleEI(testTime)
		assert.Equal(t, oracleEci, EI(0))
	}
}
