package metrics

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/metrics"

	"github.com/iotaledger/hive.go/events"

	"github.com/magiconair/properties/assert"
)

func TestReceivedTransactionsPerSecond(t *testing.T) {
	// simulate attaching 10 value payloads in 0s < t < 1s
	for i := 0; i < 10; i++ {
		valueTransactionCounter.Inc()
	}

	assert.Equal(t, ValueTransactionCounter(), (uint64)(10))
}

func TestValueTips(t *testing.T) {
	var wg sync.WaitGroup
	metrics.Events().ValueTips.Attach(events.NewClosure(func(tips uint64) {
		valueTips.Store(tips)
		wg.Done()
	}))
	wg.Add(1)
	measureValueTips()
	wg.Wait()
	assert.Equal(t, ValueTips(), (uint64)(0))
}
