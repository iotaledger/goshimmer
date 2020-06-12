package metrics

import (
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)



func TestCpuUsage(t *testing.T) {
	var wg sync.WaitGroup
	metrics.Events().CPUUsage.Attach(events.NewClosure(func(cpuPercentage float64) {
		assert.NotEqual(t, 0.0, cpuPercentage)
		wg.Done()
	}))

	wg.Add(1)
	measureCPUUsage()
	wg.Wait()
}

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