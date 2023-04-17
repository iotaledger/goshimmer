package p2p

import (
	"github.com/iotaledger/hive.go/runtime/event"
)

// NeighborGroupEvents is a collection of events specific for a particular neighbors group, e.g "manual" or "auto".
type NeighborGroupEvents struct {
	// Fired when a neighbor connection has been established.
	NeighborAdded *event.Event1[*NeighborAddedEvent]

	// Fired when a neighbor has been removed.
	NeighborRemoved *event.Event1[*NeighborRemovedEvent]
}

// NewNeighborGroupEvents returns a new instance of NeighborGroupEvents.
func NewNeighborGroupEvents() *NeighborGroupEvents {
	return &NeighborGroupEvents{
		NeighborAdded:   event.New1[*NeighborAddedEvent](),
		NeighborRemoved: event.New1[*NeighborRemovedEvent](),
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
