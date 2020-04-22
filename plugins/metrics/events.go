package metrics

import (
	"github.com/iotaledger/hive.go/events"
)

var Events = pluginEvents{
	ReceivedMPSUpdated: events.NewEvent(uint64EventCaller),
}

type pluginEvents struct {
	// Fired when the messages per second metric is updated.
	ReceivedMPSUpdated *events.Event
}

func uint64EventCaller(handler interface{}, params ...interface{}) {
	handler.(func(uint64))(params[0].(uint64))
}
