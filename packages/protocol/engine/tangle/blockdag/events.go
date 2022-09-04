package blockdag

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

// Events is a collection of Tangle related Events
type Events struct {
	// BlockAttached is triggered when a previously unknown Block is attached.
	BlockAttached *event.Linkable[*Block, Events, *Events]

	// BlockSolid is triggered when a Block becomes solid (its entire past cone is known and solid).
	BlockSolid *event.Linkable[*Block, Events, *Events]

	// BlockMissing is triggered when a referenced Block was not attached, yet.
	BlockMissing *event.Linkable[*Block, Events, *Events]

	// MissingBlockAttached is triggered when a previously missing Block was attached.
	MissingBlockAttached *event.Linkable[*Block, Events, *Events]

	// BlockInvalid is triggered when a Block is found to be invalid.
	BlockInvalid *event.Linkable[*Block, Events, *Events]

	// BlockOrphaned is triggered when a Block becomes orphaned.
	BlockOrphaned *event.Linkable[*Block, Events, *Events]

	// BlockUnorphaned is triggered when a Block is no longer orphaned.
	BlockUnorphaned *event.Linkable[*Block, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableCollectionConstructor[Events](func(e *Events) {
	e.BlockAttached = event.Link(event.NewLinkable[*Block, Events](), e, func(target *Events) { e.BlockAttached.LinkTo(target.BlockAttached) })
	e.BlockSolid = event.Link(event.NewLinkable[*Block, Events](), e, func(target *Events) { e.BlockSolid.LinkTo(target.BlockSolid) })
	e.BlockMissing = event.Link(event.NewLinkable[*Block, Events](), e, func(target *Events) { e.BlockMissing.LinkTo(target.BlockMissing) })
	e.MissingBlockAttached = event.Link(event.NewLinkable[*Block, Events](), e, func(target *Events) { e.MissingBlockAttached.LinkTo(target.MissingBlockAttached) })
	e.BlockInvalid = event.Link(event.NewLinkable[*Block, Events](), e, func(target *Events) { e.BlockInvalid.LinkTo(target.BlockInvalid) })
	e.BlockOrphaned = event.Link(event.NewLinkable[*Block, Events](), e, func(target *Events) { e.BlockOrphaned.LinkTo(target.BlockOrphaned) })
	e.BlockUnorphaned = event.Link(event.NewLinkable[*Block, Events](), e, func(target *Events) { e.BlockUnorphaned.LinkTo(target.BlockUnorphaned) })
})
