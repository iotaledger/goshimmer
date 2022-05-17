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
		// eci = 0
		testTime := genesisTime.Add(5 * time.Second)
		eci := manager.TimeToECI(testTime)
		assert.Equal(t, eci, ECI(0))

		oracleEci := manager.TimeToOracleECI(testTime)
		assert.Equal(t, oracleEci, ECI(0))

		startTime := manager.ECIToStartTime(eci)
		assert.Equal(t, startTime, time.Unix(genesisTime.Unix(), 0))
		endTime := manager.ECIToEndTime(eci)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(9*time.Second).Unix(), 0))
	}

	{
		// eci = 1
		testTime := genesisTime.Add(10 * time.Second)
		eci := manager.TimeToECI(testTime)
		assert.Equal(t, eci, ECI(1))

		oracleEci := manager.TimeToOracleECI(testTime)
		assert.Equal(t, oracleEci, ECI(0))

		startTime := manager.ECIToStartTime(eci)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(10*time.Second).Unix(), 0))
		endTime := manager.ECIToEndTime(eci)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(19*time.Second).Unix(), 0))
	}

	{
		// eci = 3
		testTime := genesisTime.Add(35 * time.Second)
		eci := manager.TimeToECI(testTime)
		assert.Equal(t, eci, ECI(3))

		oracleEci := manager.TimeToOracleECI(testTime)
		assert.Equal(t, oracleEci, ECI(0))

		startTime := manager.ECIToStartTime(eci)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(30*time.Second).Unix(), 0))
		endTime := manager.ECIToEndTime(eci)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(39*time.Second).Unix(), 0))
	}

	{
		// eci = 4
		testTime := genesisTime.Add(49 * time.Second)
		eci := manager.TimeToECI(testTime)
		assert.Equal(t, eci, ECI(4))

		oracleEci := manager.TimeToOracleECI(testTime)
		assert.Equal(t, oracleEci, ECI(2))
	}

	{
		// a time before genesis time, eci = 0
		testTime := genesisTime.Add(-10 * time.Second)
		eci := manager.TimeToECI(testTime)
		assert.Equal(t, eci, ECI(0))

		oracleEci := manager.TimeToOracleECI(testTime)
		assert.Equal(t, oracleEci, ECI(0))
	}
}
