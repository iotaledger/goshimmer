package metrics

import (
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/shirou/gopsutil/cpu"
	"runtime"
	"time"
)

var (
	_cpuUsage float64
	cpuLock syncutils.RWMutex
	_memUsageBytes uint64
	memUsageLock syncutils.RWMutex
)

func CpuUsage() float64 {
	cpuLock.RLock()
	defer cpuLock.RUnlock()
	return _cpuUsage
}

func measureCPUUsage() {
	var p float64
	//Percent calculates the percentage of cpu used either per CPU or combined.
	// TODO: use func PercentWithContext for more detailed info
	percent, err := cpu.Percent(time.Second,false)
	if err == nil {
		p = percent[0]
	}
	metrics.Events().CPUUsage.Trigger(p)
}

func measureMemUsage() {
	var m runtime.MemStats

	runtime.ReadMemStats(&m)

	metrics.Events().MemUsage.Trigger(m.Alloc)
}

func MemUsage() uint64 {
	memUsageLock.RLock()
	defer memUsageLock.RUnlock()
	return _memUsageBytes
}