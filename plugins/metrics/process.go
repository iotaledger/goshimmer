package metrics

import (
	"runtime"
	"time"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/shirou/gopsutil/cpu"
	"go.uber.org/atomic"
)

var (
	cpuUsage      atomic.Float64
	memUsageBytes atomic.Uint64
)

// CPUUsage returns the current cpu usage.
func CPUUsage() float64 {
	return cpuUsage.Load()
}

func measureCPUUsage() {
	var p float64
	// Percent calculates the percentage of cpu used either per CPU or combined.
	// TODO: use func PercentWithContext for more detailed info.
	percent, err := cpu.Percent(time.Second, false)
	if err == nil && len(percent) > 0 {
		p = percent[0]
	}
	metrics.Events().CPUUsage.Trigger(p)
}

func measureMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	metrics.Events().MemUsage.Trigger(m.Alloc)
}

// MemUsage returns the current memory allocated as bytes.
func MemUsage() uint64 {
	return memUsageBytes.Load()
}
