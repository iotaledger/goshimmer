package booker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type Events struct {
	BlockBooked         *event.Linkable[*BlockBookedEvent]
	AttachmentCreated   *event.Linkable[*Block]
	AttachmentOrphaned  *event.Linkable[*Block]
	BlockConflictAdded  *event.Linkable[*BlockConflictAddedEvent]
	MarkerConflictAdded *event.Linkable[*MarkerConflictAddedEvent]
	Error               *event.Linkable[error]

	MarkerManager *markermanager.Events

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockBooked:         event.NewLinkable[*BlockBookedEvent](),
		AttachmentCreated:   event.NewLinkable[*Block](),
		AttachmentOrphaned:  event.NewLinkable[*Block](),
		BlockConflictAdded:  event.NewLinkable[*BlockConflictAddedEvent](),
		MarkerConflictAdded: event.NewLinkable[*MarkerConflictAddedEvent](),
		Error:               event.NewLinkable[error](),

		MarkerManager: markermanager.NewEvents(),
	}
})

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

type BlockBookedEvent struct {
	Block       *Block
	ConflictIDs utxo.TransactionIDs
}
