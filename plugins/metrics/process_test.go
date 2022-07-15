package metrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/generics/event"

	metrics2 "github.com/iotaledger/goshimmer/packages/apps/metrics"
)

func TestMemUsage(t *testing.T) {
	var wg sync.WaitGroup
	metrics2.Events.MemUsage.Attach(event.NewClosure(func(event *metrics2.MemUsageEvent) {
		assert.NotEqual(t, 0, event.MemAllocBytes)
		wg.Done()
	}))

	wg.Add(1)
	measureMemUsage()
	wg.Wait()
}
