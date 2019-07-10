package metrics

import (
	"github.com/iotaledger/goshimmer/packages/events"
)

// define a struct with the available events
type pluginEvents struct {
	ReceivedTPSUpdated *events.Event
}

// define a function that maps our event parameters to the correct type: uint64 (GO has no generics)
func uint64EventCaller(handler interface{}, params ...interface{}) {
	handler.(func(uint64))(params[0].(uint64))
}

// expose our event API
var Events = pluginEvents{
	ReceivedTPSUpdated: events.NewEvent(uint64EventCaller),
}
