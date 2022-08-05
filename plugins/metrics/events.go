package metrics

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
)

// Events defines the events of the plugin.
var Events *EventsStruct

type EventsStruct struct {
	// Fired when the blocks per second metric is updated.
	ReceivedBPSUpdated *event.Event[*ReceivedBPSUpdatedEvent]
	// Fired when the transactions per second metric is updated.
	ReceivedTPSUpdated *event.Event[*ReceivedTPSUpdatedEvent]
	// Fired when the component counter per second metric is updated.
	ComponentCounterUpdated *event.Event[*ComponentCounterUpdatedEvent]
	// RateSetterUpdated is fired when the rate setter metric is updated.
	RateSetterUpdated *event.Event[*RateSetterMetric]
}

func newEvents() (new *EventsStruct) {
	return &EventsStruct{
		ReceivedBPSUpdated:      event.New[*ReceivedBPSUpdatedEvent](),
		ReceivedTPSUpdated:      event.New[*ReceivedTPSUpdatedEvent](),
		ComponentCounterUpdated: event.New[*ComponentCounterUpdatedEvent](),
		RateSetterUpdated:       event.New[*RateSetterMetric](),
	}
}

func init() {
	Events = newEvents()
}

type ReceivedBPSUpdatedEvent struct {
	BPS uint64
}

// RateSetterMetric is the metric for the rate setter.
type RateSetterMetric struct {
	Size     int
	Estimate time.Duration
	Rate     float64
}

type ReceivedTPSUpdatedEvent struct {
	TPS uint64
}

type ComponentCounterUpdatedEvent struct {
	ComponentStatus map[ComponentType]uint64
}
