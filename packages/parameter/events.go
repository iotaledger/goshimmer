package parameter

import (
	"github.com/iotaledger/goshimmer/packages/events"
)

var Events = struct {
	AddInt    *events.Event
	AddString *events.Event
}{
	events.NewEvent(intParameterCaller),
	events.NewEvent(stringParameterCaller),
}

func intParameterCaller(handler interface{}, params ...interface{}) {
	handler.(func(*IntParameter))(params[0].(*IntParameter))
}
func stringParameterCaller(handler interface{}, params ...interface{}) {
	handler.(func(*StringParameter))(params[0].(*StringParameter))
}
