package booker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

type Events struct {
	BlockBooked         *event.Event[*Block]
	BlockConflictAdded  *event.Event[*BlockConflictAddedEvent]
	MarkerConflictAdded *event.Event[*MarkerConflictAddedEvent]
	Error               *event.Event[error]
}

type BlockConflictAddedEvent struct {
	Block             *Block
	ConflictID        utxo.TransactionID
	ParentConflictIDs utxo.TransactionIDs
}

type MarkerConflictAddedEvent struct {
	Marker            markers.Marker
	ConflictID        utxo.TransactionID
	ParentConflictIDs utxo.TransactionIDs
}

// newEvents creates a new Events instance.
func newEvents() *Events {
	return &Events{
		BlockBooked:         event.New[*Block](),
		BlockConflictAdded:  event.New[*BlockConflictAddedEvent](),
		MarkerConflictAdded: event.New[*MarkerConflictAddedEvent](),
		Error:               event.New[error](),
	}
}
