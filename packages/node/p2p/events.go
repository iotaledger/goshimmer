package p2p

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/libp2p/go-libp2p-core/protocol"
	"google.golang.org/protobuf/proto"
)

// NeighborGroupEvents is a collection of events specific for a particular neighbors group, e.g "manual" or "auto".
type NeighborGroupEvents struct {
	// Fired when a neighbor connection has been established.
	NeighborAdded *event.Event[*NeighborAddedEvent]

	// Fired when a neighbor has been removed.
	NeighborRemoved *event.Event[*NeighborRemovedEvent]
}

// NewNeighborGroupEvents returns a new instance of NeighborGroupEvents.
func NewNeighborGroupEvents() (new *NeighborGroupEvents) {
	return &NeighborGroupEvents{
		NeighborAdded:   event.New[*NeighborAddedEvent](),
		NeighborRemoved: event.New[*NeighborRemovedEvent](),
	}
}

// NeighborAddedEvent holds data about the added neighbor.
type NeighborAddedEvent struct {
	Neighbor *Neighbor
}

// NeighborRemovedEvent holds data about the removed neighbor.
type NeighborRemovedEvent struct {
	Neighbor *Neighbor
}

// NeighborEvents is a collection of events specific to a neighbor.
type NeighborEvents struct {
	// Fired when a neighbor disconnects.
	Disconnected   *event.Event[*NeighborDisconnectedEvent]
	PacketReceived *event.Event[*NeighborPacketReceivedEvent]
}

// NewNeighborEvents returns a new instance of NeighborEvents.
func NewNeighborEvents() (new *NeighborEvents) {
	return &NeighborEvents{
		Disconnected:   event.New[*NeighborDisconnectedEvent](),
		PacketReceived: event.New[*NeighborPacketReceivedEvent](),
	}
}

// NeighborDisconnectedEvent holds data about the disconnected neighbor.
type NeighborDisconnectedEvent struct{}

// NeighborPacketReceivedEvent holds data about a protocol and packet received from a neighbor.
type NeighborPacketReceivedEvent struct {
	Neighbor *Neighbor
	Protocol protocol.ID
	Packet   proto.Message
}
