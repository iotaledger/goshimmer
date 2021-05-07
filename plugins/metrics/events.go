package metrics

import (
	"github.com/iotaledger/hive.go/events"
)

// Events defines the events of the plugin.
var Events = pluginEvents{
	// ReceivedMPSUpdated triggers upon reception of a MPS update.
	ReceivedMPSUpdated:      events.NewEvent(uint64EventCaller),
	ReceivedTPSUpdated:      events.NewEvent(uint64EventCaller),
	ComponentCounterUpdated: events.NewEvent(componentTypeUint64EventCaller),
}

type pluginEvents struct {
	// Fired when the messages per second metric is updated.
	ReceivedMPSUpdated *events.Event
	// Fired when the transactions per second metric is updated.
	ReceivedTPSUpdated *events.Event
	// Fired when the component counter per second metric is updated.
	ComponentCounterUpdated *events.Event
}

func uint64EventCaller(handler interface{}, params ...interface{}) {
	handler.(func(uint64))(params[0].(uint64))
}

func componentTypeUint64EventCaller(handler interface{}, params ...interface{}) {
	handler.(func(map[ComponentType]uint64))(params[0].(map[ComponentType]uint64))
}
