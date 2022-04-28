package metrics

import "github.com/iotaledger/hive.go/generics/event"

// Events defines the events of the plugin.
var Events *EventsStruct

type EventsStruct struct {
	// Fired when the messages per second metric is updated.
	ReceivedMPSUpdated *event.Event[*ReceivedMPSUpdatedEvent]
	// Fired when the transactions per second metric is updated.
	ReceivedTPSUpdated *event.Event[*ReceivedTPSUpdatedEvent]
	// Fired when the component counter per second metric is updated.
	ComponentCounterUpdated *event.Event[*ComponentCounterUpdatedEvent]
}

func newEvents() (new *EventsStruct) {
	return &EventsStruct{
		ReceivedMPSUpdated:      event.New[*ReceivedMPSUpdatedEvent](),
		ReceivedTPSUpdated:      event.New[*ReceivedTPSUpdatedEvent](),
		ComponentCounterUpdated: event.New[*ComponentCounterUpdatedEvent](),
	}

}

func init() {
	Events = newEvents()
}

type ReceivedMPSUpdatedEvent struct {
	MPS uint64
}

type ReceivedTPSUpdatedEvent struct {
	TPS uint64
}

type ComponentCounterUpdatedEvent struct {
	ComponentStatus map[ComponentType]uint64
}
