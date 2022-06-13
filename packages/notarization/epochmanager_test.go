package notarization

import (
	"testing"
	"time"

	"github.com/magiconair/properties/assert"

	"github.com/iotaledger/goshimmer/packages/epoch"
)

func TestEpochManager(t *testing.T) {
	genesisTime := time.Now()
	manager := NewEpochManager(GenesisTime(genesisTime.Unix()), Duration(10*time.Second))

	{
		// ei = 0
		testTime := genesisTime.Add(5 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, epoch.Index(0))

		startTime := manager.EIToStartTime(ei)
		assert.Equal(t, startTime, time.Unix(genesisTime.Unix(), 0))
		endTime := manager.EIToEndTime(ei)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(9*time.Second).Unix(), 0))
	}

	{
		// ei = 1
		testTime := genesisTime.Add(10 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, epoch.Index(1))

		startTime := manager.EIToStartTime(ei)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(10*time.Second).Unix(), 0))
		endTime := manager.EIToEndTime(ei)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(19*time.Second).Unix(), 0))
	}

	{
		// ei = 3
		testTime := genesisTime.Add(35 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, epoch.Index(3))

		startTime := manager.EIToStartTime(ei)
		assert.Equal(t, startTime, time.Unix(genesisTime.Add(30*time.Second).Unix(), 0))
		endTime := manager.EIToEndTime(ei)
		assert.Equal(t, endTime, time.Unix(genesisTime.Add(39*time.Second).Unix(), 0))
	}

	{
		// ei = 4
		testTime := genesisTime.Add(49 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, epoch.Index(4))

	}

	{
		// a time before genesis time, ei = 0
		testTime := genesisTime.Add(-10 * time.Second)
		ei := manager.TimeToEI(testTime)
		assert.Equal(t, ei, epoch.Index(0))
	}
}
