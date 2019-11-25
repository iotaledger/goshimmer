package metrics

import (
	"github.com/iotaledger/hive.go/events"
)

var Events = pluginEvents{
	ReceivedTPSUpdated: events.NewEvent(uint64EventCaller),
}

type pluginEvents struct {
	ReceivedTPSUpdated *events.Event
}

func uint64EventCaller(handler interface{}, params ...interface{}) {
	handler.(func(uint64))(params[0].(uint64))
}
