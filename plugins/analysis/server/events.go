package server

import (
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
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
	// FPCHeartbeat triggers when an FPC heartbeat has been received.
	FPCHeartbeat *events.Event
	// MetricHeartbeat triggers when an MetricHeartbeat heartbeat has been received.
	MetricHeartbeat *events.Event
}{
	events.NewEvent(stringCaller),
	events.NewEvent(stringCaller),
	events.NewEvent(stringStringCaller),
	events.NewEvent(stringStringCaller),
	events.NewEvent(errorCaller),
	events.NewEvent(heartbeatPacketCaller),
	events.NewEvent(fpcHeartbeatPacketCaller),
	events.NewEvent(metricHeartbeatPacketCaller),
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
	handler.(func(heartbeat *packet.Heartbeat))(params[0].(*packet.Heartbeat))
}

func fpcHeartbeatPacketCaller(handler interface{}, params ...interface{}) {
	handler.(func(hb *packet.FPCHeartbeat))(params[0].(*packet.FPCHeartbeat))
}

func metricHeartbeatPacketCaller(handler interface{}, params ...interface{}) {
	handler.(func(hb *packet.MetricHeartbeat))(params[0].(*packet.MetricHeartbeat))
}
