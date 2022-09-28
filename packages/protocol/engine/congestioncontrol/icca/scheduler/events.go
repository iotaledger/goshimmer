package scheduler

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Scheduler.
type Events struct {
	// BlockScheduled is triggered when a block is ready to be scheduled.
	BlockScheduled *event.Linkable[*Block, Events, *Events]
	// BlockDropped is triggered when a block is removed from the longest mana-scaled queue when the buffer is full.
	BlockDropped *event.Linkable[*Block, Events, *Events]
	// BlockSkipped is triggered when a block is confirmed before it's scheduled, and is skipped by the scheduler.
	BlockSkipped *event.Linkable[*Block, Events, *Events]
	Error        *event.Linkable[error, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockScheduled: event.NewLinkable[*Block, Events, *Events](),
		BlockDropped:   event.NewLinkable[*Block, Events, *Events](),
		BlockSkipped:   event.NewLinkable[*Block, Events, *Events](),
		Error:          event.NewLinkable[error, Events, *Events](),
	}
})

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
