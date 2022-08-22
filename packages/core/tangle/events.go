package tangle

import "github.com/iotaledger/hive.go/core/generics/event"

// Events is a collection of Tangle related Events
type Events struct {
	// BlockAttached is triggered when a previously unknown Block is attached.
	BlockAttached *event.Event[*Block]

	// BlockSolid is triggered when a Block becomes solid (its entire past cone is known and solid).
	BlockSolid *event.Event[*Block]

	// BlockMissing is triggered when a referenced Block was not attached, yet.
	BlockMissing *event.Event[*Block]

	// MissingBlockAttached is triggered when a previously missing Block was attached.
	MissingBlockAttached *event.Event[*Block]

	// BlockInvalid is triggered when a Block is found to be invalid.
	BlockInvalid *event.Event[*Block]

	// BlockOrphaned is triggered when a Block becomes orphaned.
	BlockOrphaned *event.Event[*Block]

	// BlockUnorphaned is triggered when a Block is no longer orphaned.
	BlockUnorphaned *event.Event[*Block]
}

// newEvents creates a new Events instance.
func newEvents() *Events {
	return &Events{
		BlockAttached:        event.New[*Block](),
		BlockSolid:           event.New[*Block](),
		BlockMissing:         event.New[*Block](),
		MissingBlockAttached: event.New[*Block](),
		BlockInvalid:         event.New[*Block](),
		BlockOrphaned:        event.New[*Block](),
		BlockUnorphaned:      event.New[*Block](),
	}
}
