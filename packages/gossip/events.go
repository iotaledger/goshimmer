package gossip

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events defines all the events related to the gossip protocol.
type Events struct {
	// Fired when a new message was received via the gossip protocol.
	MessageReceived *events.Event
}

// NeighborsEvents is a collection of events specific for a particular neighbors group, e.g "manual" or "auto".
type NeighborsEvents struct {
	// Fired when a neighbor connection has been established.
	NeighborAdded *events.Event
	// Fired when a neighbor has been removed.
	NeighborRemoved *events.Event
}

// NewNeighborsEvents returns a new instance of NeighborsEvents.
func NewNeighborsEvents() NeighborsEvents {
	return NeighborsEvents{
		NeighborAdded:   events.NewEvent(neighborCaller),
		NeighborRemoved: events.NewEvent(neighborCaller),
	}
}

// MessageReceivedEvent holds data about a message received event.
type MessageReceivedEvent struct {
	// The raw message.
	Data []byte
	// The sender of the message.
	Peer *peer.Peer
}

func neighborCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Neighbor))(params[0].(*Neighbor))
}

func messageReceived(handler interface{}, params ...interface{}) {
	handler.(func(*MessageReceivedEvent))(params[0].(*MessageReceivedEvent))
}
