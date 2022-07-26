package tangle

import "github.com/iotaledger/hive.go/generics/event"

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is the event interface of the Tangle.
type Events struct {
	Error *event.Event[error]

	// BlockStored is triggered when a block is stored.
	BlockStored *event.Event[*BlockMetadata]

	// BlockSolid is triggered when a block becomes solid, i.e. its past cone is known and solid.
	BlockSolid *event.Event[*BlockMetadata]

	// BlockMissing is triggered when a block references an unknown parent Block.
	BlockMissing *event.Event[*BlockMetadata]

	// MissingBlockStored fired when a block which was previously marked as missing was received.
	MissingBlockStored *event.Event[*BlockMetadata]

	// MissingBlockStored fired when a block which was previously marked as missing was invalid.
	BlockInvalid *event.Event[*BlockMetadata]
}

func newEvents() *Events {
	return &Events{
		Error:              event.New[error](),
		BlockStored:        event.New[*BlockMetadata](),
		BlockSolid:         event.New[*BlockMetadata](),
		BlockMissing:       event.New[*BlockMetadata](),
		MissingBlockStored: event.New[*BlockMetadata](),
		BlockInvalid:       event.New[*BlockMetadata](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
