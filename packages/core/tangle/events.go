package tangle

import "github.com/iotaledger/hive.go/generics/event"

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is the event interface of the Tangle.
type Events struct {
	Error *event.Event[error]

	// BlockSolid is triggered when a block becomes solid, i.e. its past cone is known and solid.
	BlockSolid *event.Event[*BlockSolidEvent]

	// BlockMissing is triggered when a block references an unknown parent Block.
	BlockMissing *event.Event[*BlockMissingEvent]

	// MissingBlockStored fired when a block which was previously marked as missing was received.
	MissingBlockStored *event.Event[*MissingBlockStoredEvent]
}

func newEvents() *Events {
	return &Events{
		Error:              event.New[error](),
		BlockSolid:         event.New[*BlockSolidEvent](),
		BlockMissing:       event.New[*BlockMissingEvent](),
		MissingBlockStored: event.New[*MissingBlockStoredEvent](),
	}
}

type BlockSolidEvent struct {
	BlockMetadata *BlockMetadata
}

type BlockMissingEvent struct {
	BlockID BlockID
}
type MissingBlockStoredEvent struct {
	BlockID BlockID
}
