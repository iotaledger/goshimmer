package server

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
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
	// MetricHeartbeat triggers when an MetricHeartbeat heartbeat has been received.
	MetricHeartbeat *events.Event
}{
	events.NewEvent(addNodeCaller),
	events.NewEvent(removeNodeCaller),
	events.NewEvent(connectNodesCaller),
	events.NewEvent(disconnectNodesCaller),
	events.NewEvent(errorCaller),
	events.NewEvent(heartbeatPacketCaller),
	events.NewEvent(metricHeartbeatPacketCaller),
}

// AddNodeEvent is the payload type of an AddNode event.
type AddNodeEvent struct {
	NetworkVersion string
	NodeID         string
}

// RemoveNodeEvent is the payload type of a RemoveNode event.
type RemoveNodeEvent struct {
	NetworkVersion string
	NodeID         string
}

// ConnectNodesEvent is the payload type of a ConnectNodesEvent.
type ConnectNodesEvent struct {
	NetworkVersion string
	SourceID       string
	TargetID       string
}

// DisconnectNodesEvent is the payload type f a DisconnectNodesEvent.
type DisconnectNodesEvent struct {
	NetworkVersion string
	SourceID       string
	TargetID       string
}

func addNodeCaller(handler interface{}, params ...interface{}) {
	handler.(func(*AddNodeEvent))(params[0].(*AddNodeEvent))
}

func removeNodeCaller(handler interface{}, params ...interface{}) {
	handler.(func(*RemoveNodeEvent))(params[0].(*RemoveNodeEvent))
}

func connectNodesCaller(handler interface{}, params ...interface{}) {
	handler.(func(*ConnectNodesEvent))(params[0].(*ConnectNodesEvent))
}

func disconnectNodesCaller(handler interface{}, params ...interface{}) {
	handler.(func(*DisconnectNodesEvent))(params[0].(*DisconnectNodesEvent))
}

func errorCaller(handler interface{}, params ...interface{}) {
	handler.(func(error))(params[0].(error))
}

func heartbeatPacketCaller(handler interface{}, params ...interface{}) {
	handler.(func(heartbeat *packet.Heartbeat))(params[0].(*packet.Heartbeat))
}

func metricHeartbeatPacketCaller(handler interface{}, params ...interface{}) {
	handler.(func(hb *packet.MetricHeartbeat))(params[0].(*packet.MetricHeartbeat))
}
