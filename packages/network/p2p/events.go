package p2p

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

// NeighborGroupEvents is a collection of events specific for a particular neighbors group, e.g "manual" or "auto".
type NeighborGroupEvents struct {
	// Fired when a neighbor connection has been established.
	NeighborAdded *event.Event[*NeighborAddedEvent]

	// Fired when a neighbor has been removed.
	NeighborRemoved *event.Event[*NeighborRemovedEvent]
}

// NewNeighborGroupEvents returns a new instance of NeighborGroupEvents.
func NewNeighborGroupEvents() *NeighborGroupEvents {
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
