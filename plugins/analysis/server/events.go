package server

import (
	"github.com/iotaledger/goshimmer/plugins/analysis/types/heartbeat"
	"github.com/iotaledger/hive.go/events"
)

// Events holds the events of the analysis server package.
var Events = struct {
	// AddNode triggers when adding a new node.
	AddNode *events.Event
	// RemoveNode triggers when removing a node.
	RemoveNode *events.Event
	// ConnectNodes triggers when connecting two nodes.
	ConnectNodes *events.Event
	// DisconnectNodes triggers when disconnecting two nodes.
	DisconnectNodes *events.Event
	// Error triggers when an error occurs.
	Error *events.Event
	// Heartbeat triggers when an heartbeat has been received.
	Heartbeat *events.Event
}{
	events.NewEvent(stringCaller),
	events.NewEvent(stringCaller),
	events.NewEvent(stringStringCaller),
	events.NewEvent(stringStringCaller),
	events.NewEvent(errorCaller),
	events.NewEvent(heartbeatPacketCaller),
}

func stringCaller(handler interface{}, params ...interface{}) {
	handler.(func(string))(params[0].(string))
}

func stringStringCaller(handler interface{}, params ...interface{}) {
	handler.(func(string, string))(params[0].(string), params[1].(string))
}

func errorCaller(handler interface{}, params ...interface{}) {
	handler.(func(error))(params[0].(error))
}

func heartbeatPacketCaller(handler interface{}, params ...interface{}) {
	handler.(func(heartbeat.Packet))(params[0].(heartbeat.Packet))
}
