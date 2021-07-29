package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
)

var (
	once         sync.Once
	metricEvents *CollectionEvents
)

func newEvents() *CollectionEvents {
	return &CollectionEvents{
		AnalysisOutboundBytes: events.NewEvent(uint64Caller),
		CPUUsage:              events.NewEvent(float64Caller),
		MemUsage:              events.NewEvent(uint64Caller),
		TangleTimeSynced:      events.NewEvent(boolCaller),
		ValueTips:             events.NewEvent(uint64Caller),
		MessageTips:           events.NewEvent(uint64Caller),
	}
}

// Events returns the events defined in the package.
func Events() *CollectionEvents {
	once.Do(func() {
		metricEvents = newEvents()
	})
	return metricEvents
}
