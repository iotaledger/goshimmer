package parameter

import (
	"github.com/iotaledger/goshimmer/packages/events"
)

var Events = struct {
	AddBool   *events.Event
	AddInt    *events.Event
	AddString *events.Event
}{
	AddBool:   events.NewEvent(boolParameterCaller),
	AddInt:    events.NewEvent(intParameterCaller),
	AddString: events.NewEvent(stringParameterCaller),
}

func boolParameterCaller(handler interface{}, params ...interface{}) {
	handler.(func(*BoolParameter))(params[0].(*BoolParameter))
}

func intParameterCaller(handler interface{}, params ...interface{}) {
	handler.(func(*IntParameter))(params[0].(*IntParameter))
}

func stringParameterCaller(handler interface{}, params ...interface{}) {
	handler.(func(*StringParameter))(params[0].(*StringParameter))
}
