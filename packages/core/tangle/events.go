package tangle

import "github.com/iotaledger/hive.go/generics/event"

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is the event interface of the Tangle.
type Events struct {
	Error *event.Event[error]

	// BlockStored is triggered when a block is stored.
	BlockStored *event.Event[*SolidifiedBlock]

	// BlockSolid is triggered when a block becomes solid, i.e. its past cone is known and solid.
	BlockSolid *event.Event[*SolidifiedBlock]

	// BlockMissing is triggered when a block references an unknown parent Block.
	BlockMissing *event.Event[*SolidifiedBlock]

	// MissingBlockStored fired when a block which was previously marked as missing was received.
	MissingBlockStored *event.Event[*SolidifiedBlock]

	// MissingBlockStored fired when a block which was previously marked as missing was invalid.
	BlockInvalid *event.Event[*SolidifiedBlock]
}

func newEvents() *Events {
	return &Events{
		Error:              event.New[error](),
		BlockStored:        event.New[*SolidifiedBlock](),
		BlockSolid:         event.New[*SolidifiedBlock](),
		BlockMissing:       event.New[*SolidifiedBlock](),
		MissingBlockStored: event.New[*SolidifiedBlock](),
		BlockInvalid:       event.New[*SolidifiedBlock](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
