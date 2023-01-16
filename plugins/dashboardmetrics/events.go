package dashboardmetrics

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
)

// Events defines the events of the plugin.
var Events *EventsStruct

type EventsStruct struct {
	// Fired when the blocks per second metric is updated.
	AttachedBPSUpdated *event.Event[*AttachedBPSUpdatedEvent]
	// Fired when the component counter per second metric is updated.
	ComponentCounterUpdated *event.Event[*ComponentCounterUpdatedEvent]
}

func newEvents() *EventsStruct {
	return &EventsStruct{
		AttachedBPSUpdated:      event.New[*AttachedBPSUpdatedEvent](),
		ComponentCounterUpdated: event.New[*ComponentCounterUpdatedEvent](),
	}
}

func init() {
	Events = newEvents()
}

type AttachedBPSUpdatedEvent struct {
	BPS uint64
}

// RateSetterMetric is the metric for the rate setter.
type RateSetterMetric struct {
	Size     int
	Estimate time.Duration
	Rate     float64
}

type ComponentCounterUpdatedEvent struct {
	ComponentStatus map[ComponentType]uint64
}
