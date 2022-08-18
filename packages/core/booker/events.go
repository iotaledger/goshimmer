package booker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

type Events struct {
	BlockBooked          *event.Event[*Block]
	BlockConflictUpdated *event.Event[*BlockConflictUpdatedEvent]
	MarkerConflictAdded  *event.Event[*MarkerConflictAddedEvent]
	Error                *event.Event[error]
}

type BlockConflictUpdatedEvent struct {
	Block      *Block
	ConflictID utxo.TransactionID
}

type MarkerConflictAddedEvent struct {
	Marker        markers.Marker
	NewConflictID utxo.TransactionID
}

// newEvents creates a new Events instance.
func newEvents() *Events {
	return &Events{
		BlockBooked:          event.New[*Block](),
		BlockConflictUpdated: event.New[*BlockConflictUpdatedEvent](),
		MarkerConflictAdded:  event.New[*MarkerConflictAddedEvent](),
		Error:                event.New[error](),
	}
}
