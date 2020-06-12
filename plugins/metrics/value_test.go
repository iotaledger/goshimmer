package metrics

import (
	"sync"
	"testing"

	"github.com/iotaledger/hive.go/events"

	"github.com/magiconair/properties/assert"
)

func TestReceivedTransactionsPerSecond(t *testing.T) {
	// simulate attaching 10 value payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increaseReceivedTPSCounter()
	}

	// first measurement happens at t=1s
	measureReceivedTPS()

	// simulate 5 TPS for 1s < t < 2s
	for i := 0; i < 5; i++ {
		increaseReceivedTPSCounter()
	}

	assert.Equal(t, ReceivedTransactionsPerSecond(), (uint64)(10))
	// measure at t=2s
	measureReceivedTPS()
	assert.Equal(t, ReceivedTransactionsPerSecond(), (uint64)(5))
	// measure at t=3s
	measureReceivedTPS()
	assert.Equal(t, ReceivedTransactionsPerSecond(), (uint64)(0))
}

func TestReceivedTPSUpdatedEvent(t *testing.T) {
	var wg sync.WaitGroup
	Events.ReceivedTPSUpdated.Attach(events.NewClosure(func(tps uint64) {
		assert.Equal(t, tps, (uint64)(10))
		wg.Done()
	}))
	// simulate attaching 10 value payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		increaseReceivedTPSCounter()
	}
	wg.Add(1)
	measureReceivedTPS()
	wg.Wait()
}
