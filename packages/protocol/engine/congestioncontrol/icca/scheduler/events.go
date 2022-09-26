package scheduler

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Scheduler.
type Events struct {
	// BlockScheduled is triggered when a block is ready to be scheduled.
	BlockScheduled *event.Event[*Block]
	// BlockDropped is triggered when a block is removed from the longest mana-scaled queue when the buffer is full.
	BlockDropped *event.Event[*Block]
	// BlockSkipped is triggered when a block is confirmed before it's scheduled, and is skipped by the scheduler.
	BlockSkipped *event.Event[*Block]
	Error        *event.Event[error]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockScheduled: event.New[*Block](),
		BlockDropped:   event.New[*Block](),
		BlockSkipped:   event.New[*Block](),
		Error:          event.New[error](),
	}
})

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
