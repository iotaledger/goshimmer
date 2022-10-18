package scheduler

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Scheduler.
type Events struct {
	// BlockScheduled is triggered when a block is ready to be scheduled.
	BlockScheduled *event.Linkable[*Block]
	// BlockDropped is triggered when a block is removed from the longest mana-scaled queue when the buffer is full.
	BlockDropped *event.Linkable[*Block]
	// BlockSkipped is triggered when a block is confirmed before it's scheduled, and is skipped by the scheduler.
	BlockSkipped *event.Linkable[*Block]
	Error        *event.Linkable[error]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockScheduled: event.NewLinkable[*Block](),
		BlockDropped:   event.NewLinkable[*Block](),
		BlockSkipped:   event.NewLinkable[*Block](),
		Error:          event.NewLinkable[error](),
	}
})

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
