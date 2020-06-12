package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
)

var (
	once         sync.Once
	metricEvents *CollectionEvents
)

func new() *CollectionEvents {
	return &CollectionEvents{
		FPCInboundBytes:  events.NewEvent(uint64Caller),
		FPCOutboundBytes: events.NewEvent(uint64Caller),
		CPUUsage:         events.NewEvent(float64Caller),
		MemUsage:         events.NewEvent(uint64Caller),
		DBSize:           events.NewEvent(uint64Caller),
		Synced:           events.NewEvent(boolCaller),
	}
}

// Events returns the events defined in the package
func Events() *CollectionEvents {
	once.Do(func() {
		metricEvents = new()
	})
	return metricEvents
}
