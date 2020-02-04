package server

import (
	"github.com/iotaledger/hive.go/events"
)

var Events = struct {
	AddNode         *events.Event
	RemoveNode      *events.Event
	ConnectNodes    *events.Event
	DisconnectNodes *events.Event
	NodeOnline      *events.Event
	NodeOffline     *events.Event
	Error           *events.Event
}{
	events.NewEvent(stringCaller),
	events.NewEvent(stringCaller),
	events.NewEvent(stringStringCaller),
	events.NewEvent(stringStringCaller),
	events.NewEvent(stringCaller),
	events.NewEvent(stringCaller),
	events.NewEvent(errorCaller),
}

func stringCaller(handler interface{}, params ...interface{}) {
	handler.(func(string))(params[0].(string))
}
func stringStringCaller(handler interface{}, params ...interface{}) {
	handler.(func(string, string))(params[0].(string), params[1].(string))
}
func errorCaller(handler interface{}, params ...interface{}) { handler.(func(error))(params[0].(error)) }
