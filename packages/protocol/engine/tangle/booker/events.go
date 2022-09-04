package booker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type Events struct {
	BlockBooked         *event.LinkableCollectionEvent[*Block, Events, *Events]
	BlockConflictAdded  *event.LinkableCollectionEvent[*BlockConflictAddedEvent, Events, *Events]
	MarkerConflictAdded *event.LinkableCollectionEvent[*MarkerConflictAddedEvent, Events, *Events]
	SequenceEvicted     *event.LinkableCollectionEvent[markers.SequenceID, Events, *Events]
	Error               *event.LinkableCollectionEvent[error, Events, *Events]

	MarkerManager *markermanager.Events

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableCollectionConstructor[Events](func(e *Events) {
	e.BlockBooked = event.NewLinkableCollectionEvent[*Block](e, func(target *Events) { e.BlockBooked.LinkTo(target.BlockBooked) })
	e.BlockConflictAdded = event.NewLinkableCollectionEvent[*BlockConflictAddedEvent](e, func(target *Events) { e.BlockConflictAdded.LinkTo(target.BlockConflictAdded) })
	e.MarkerConflictAdded = event.NewLinkableCollectionEvent[*MarkerConflictAddedEvent](e, func(target *Events) { e.MarkerConflictAdded.LinkTo(target.MarkerConflictAdded) })
	e.SequenceEvicted = event.NewLinkableCollectionEvent[markers.SequenceID](e, func(target *Events) { e.SequenceEvicted.LinkTo(target.SequenceEvicted) })
	e.Error = event.NewLinkableCollectionEvent[error](e, func(target *Events) { e.Error.LinkTo(target.Error) })

	e.MarkerManager = markermanager.NewEvents()

	e.OnLinkUpdated(func(linkTarget *Events) {
		e.MarkerManager.LinkTo(linkTarget.MarkerManager)
	})
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
