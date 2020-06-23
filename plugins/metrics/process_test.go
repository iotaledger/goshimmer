package metrics

import (
	"sync"
	"testing"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestMemUsage(t *testing.T) {
	var wg sync.WaitGroup
	metrics.Events().MemUsage.Attach(events.NewClosure(func(memUsageBytes uint64) {
		assert.NotEqual(t, 0, memUsageBytes)
		wg.Done()
	}))

	wg.Add(1)
	measureMemUsage()
	wg.Wait()
}
