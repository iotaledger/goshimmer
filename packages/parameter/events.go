package parameter

import (
	"github.com/iotaledger/hive.go/events"
)

var Events = struct {
	AddBool   *events.Event
	AddInt    *events.Event
	AddString *events.Event
	AddPlugin *events.Event
}{
	AddBool:   events.NewEvent(boolParameterCaller),
	AddInt:    events.NewEvent(intParameterCaller),
	AddString: events.NewEvent(stringParameterCaller),
	AddPlugin: events.NewEvent(pluginParameterCaller),
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

func pluginParameterCaller(handler interface{}, params ...interface{}) {
	handler.(func(string, int))(params[0].(string), params[1].(int))
}
