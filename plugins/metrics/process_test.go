package metrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/hive.go/generics/event"
)

func TestMemUsage(t *testing.T) {
	var wg sync.WaitGroup
	metrics.Events.MemUsage.Attach(event.NewClosure(func(event *metrics.MemUsageEvent) {
		assert.NotEqual(t, 0, event.MemAllocBytes)
		wg.Done()
	}))

	wg.Add(1)
	measureMemUsage()
	wg.Wait()
}
