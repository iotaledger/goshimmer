package dashboardmetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/hive.go/runtime/event"
)

// Events defines the events of the plugin.
var Events *EventsStruct

type EventsStruct struct {
	// Fired when the blocks per second metric is updated.
	AttachedBPSUpdated *event.Event1[*AttachedBPSUpdatedEvent]
	// Fired when the component counter per second metric is updated.
	ComponentCounterUpdated *event.Event1[*ComponentCounterUpdatedEvent]
	// RateSetterUpdated is fired when the rate setter metric is updated.
	RateSetterUpdated *event.Event1[*RateSetterMetric]
}

func newEvents() *EventsStruct {
	return &EventsStruct{
		AttachedBPSUpdated:      event.New1[*AttachedBPSUpdatedEvent](),
		ComponentCounterUpdated: event.New1[*ComponentCounterUpdatedEvent](),
		RateSetterUpdated:       event.New1[*RateSetterMetric](),
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
	ComponentStatus map[collector.ComponentType]uint64
}
