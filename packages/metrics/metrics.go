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
		events.NewEvent(uint64Caller),
		events.NewEvent(uint64Caller),
		events.NewEvent(float64Caller),
		events.NewEvent(uint64Caller),
		events.NewEvent(boolCaller),
		events.NewEvent(uint64Caller),
		events.NewEvent(uint64Caller),
		events.NewEvent(queryReceivedEventCaller),
		events.NewEvent(queryReplyErrorEventCaller),
	}
}

// Events returns the events defined in the package
func Events() *CollectionEvents {
	once.Do(func() {
		metricEvents = new()
	})
	return metricEvents
}
