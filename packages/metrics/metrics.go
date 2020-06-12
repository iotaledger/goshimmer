package metrics

import (
	"github.com/iotaledger/hive.go/events"
	"sync"
)

var (
	once sync.Once
	metricEvents *CollectionEvents
)

func new()*CollectionEvents {
	return &CollectionEvents{
		events.NewEvent(uint64Caller),
		events.NewEvent(uint64Caller),
		events.NewEvent(float64Caller),
		events.NewEvent(uint64Caller),
	}
}

// Events returns the events defined in the package
func Events() *CollectionEvents {
	once.Do(func(){
		metricEvents = new()
	})
	return metricEvents
}