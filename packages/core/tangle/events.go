package tangle

import "github.com/iotaledger/hive.go/generics/event"

// Events is the event interface of the Tangle.
type Events struct {
	// BlockStored is triggered when a block is stored.
	BlockStored *event.Event[*Block]

	// BlockSolid is triggered when a block becomes solid, i.e. its past cone is known and solid.
	BlockSolid *event.Event[*Block]

	// BlockMissing is triggered when a block references an unknown parent Block.
	BlockMissing *event.Event[*Block]

	// MissingBlockStored fired when a block which was previously marked as missing was received.
	MissingBlockStored *event.Event[*Block]

	// MissingBlockStored fired when a block which was previously marked as missing was invalid.
	BlockInvalid *event.Event[*Block]

	Error *event.Event[error]
}

func newEvents() *Events {
	return &Events{
		Error:              event.New[error](),
		BlockStored:        event.New[*Block](),
		BlockSolid:         event.New[*Block](),
		BlockMissing:       event.New[*Block](),
		MissingBlockStored: event.New[*Block](),
		BlockInvalid:       event.New[*Block](),
	}
}
