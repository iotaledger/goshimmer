package server

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
)

// EventsStruct holds the events of the analysis server package.
type EventsStruct struct {
	// AddNode triggers when adding a new node.
	AddNode *event.Event[*AddNodeEvent]
	// RemoveNode triggers when removing a node.
	RemoveNode *event.Event[*RemoveNodeEvent]
	// ConnectNodes triggers when connecting two nodes.
	ConnectNodes *event.Event[*ConnectNodesEvent]
	// DisconnectNodes triggers when disconnecting two nodes.
	DisconnectNodes *event.Event[*DisconnectNodesEvent]
	// Error triggers when an error occurs.
	Error *event.Event[error]
	// Heartbeat triggers when an heartbeat has been received.
	Heartbeat *event.Event[*HeartbeatEvent]
	// MetricHeartbeat triggers when an MetricHeartbeat heartbeat has been received.
	MetricHeartbeat *event.Event[*MetricHeartbeatEvent]
}

func newEvents() (new *EventsStruct) {
	return &EventsStruct{
		AddNode:         event.New[*AddNodeEvent](),
		RemoveNode:      event.New[*RemoveNodeEvent](),
		ConnectNodes:    event.New[*ConnectNodesEvent](),
		DisconnectNodes: event.New[*DisconnectNodesEvent](),
		Error:           event.New[error](),
		Heartbeat:       event.New[*HeartbeatEvent](),
		MetricHeartbeat: event.New[*MetricHeartbeatEvent](),
	}
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

type HeartbeatEvent struct {
	Heartbeat *packet.Heartbeat
}

type MetricHeartbeatEvent struct {
	MetricHeartbeat *packet.MetricHeartbeat
}
