package blockdag

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

// Events is a collection of Tangle related Events
type Events struct {
	// BlockAttached is triggered when a previously unknown Block is attached.
	BlockAttached *event.Linkable[*Block]

	// BlockSolid is triggered when a Block becomes solid (its entire past cone is known and solid).
	BlockSolid *event.Linkable[*Block]

	// BlockMissing is triggered when a referenced Block was not attached, yet.
	BlockMissing *event.Linkable[*Block]

	// MissingBlockAttached is triggered when a previously missing Block was attached.
	MissingBlockAttached *event.Linkable[*Block]

	// BlockInvalid is triggered when a Block is found to be invalid.
	BlockInvalid *event.Linkable[*BlockInvalidEvent]

	// BlockOrphaned is triggered when a Block becomes orphaned.
	BlockOrphaned *event.Linkable[*Block]

	// BlockUnorphaned is triggered when a Block is no longer orphaned.
	BlockUnorphaned *event.Linkable[*Block]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockAttached:        event.NewLinkable[*Block](),
		BlockSolid:           event.NewLinkable[*Block](),
		BlockMissing:         event.NewLinkable[*Block](),
		MissingBlockAttached: event.NewLinkable[*Block](),
		BlockInvalid:         event.NewLinkable[*BlockInvalidEvent](),
		BlockOrphaned:        event.NewLinkable[*Block](),
		BlockUnorphaned:      event.NewLinkable[*Block](),
	}
})

type BlockInvalidEvent struct {
	Block  *Block
	Reason error
}
