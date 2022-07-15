package gossip

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"

	pb "github.com/iotaledger/goshimmer/packages/network/gossip/gossipproto"
)

// Events defines all the events related to the gossip protocol.
type Events struct {
	// Fired when a new block was received via the gossip protocol.
	BlockReceived *event.Event[*BlockReceivedEvent]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		BlockReceived: event.New[*BlockReceivedEvent](),
	}
}

// BlockReceivedEvent holds data about a block received event.
type BlockReceivedEvent struct {
	// The raw block.
	Data []byte
	// The sender of the block.
	Peer *peer.Peer
}

// NeighborsEvents is a collection of events specific for a particular neighbors group, e.g "manual" or "auto".
type NeighborsEvents struct {
	// Fired when a neighbor connection has been established.
	NeighborAdded *event.Event[*NeighborAddedEvent]

	// Fired when a neighbor has been removed.
	NeighborRemoved *event.Event[*NeighborRemovedEvent]
}

// NewNeighborsEvents returns a new instance of NeighborsEvents.
func NewNeighborsEvents() (new *NeighborsEvents) {
	return &NeighborsEvents{
		NeighborAdded:   event.New[*NeighborAddedEvent](),
		NeighborRemoved: event.New[*NeighborRemovedEvent](),
	}
}

// NeighborAddedEvent holds data about the added neighbor.
type NeighborAddedEvent struct {
	Neighbor *Neighbor
}

// NeighborAddedEvent holds data about the removed neighbor.
type NeighborRemovedEvent struct {
	Neighbor *Neighbor
}

// NeighborEvents is a collection of events specific to a neighbor.
type NeighborEvents struct {
	// Fired when a neighbor disconnects.
	Disconnected *event.Event[*NeighborDisconnectedEvent]

	// Fired when a packet is received from a neighbor.
	PacketReceived *event.Event[*NeighborPacketReceivedEvent]
}

// NewNeighborsEvents returns a new instance of NeighborsEvents.
func NewNeighborEvents() (new *NeighborEvents) {
	return &NeighborEvents{
		Disconnected:   event.New[*NeighborDisconnectedEvent](),
		PacketReceived: event.New[*NeighborPacketReceivedEvent](),
	}
}

// NeighborDisconnectedEvent holds data about the disconnected neighbor.
type NeighborDisconnectedEvent struct{}

// NeighborDisconnectedEvent holds data about the disconnected neighbor.
type NeighborPacketReceivedEvent struct {
	Packet *pb.Packet
}
