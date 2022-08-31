package scheduler

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Scheduler.
type Events struct {
	// BlockScheduled is triggered when a block is ready to be scheduled.
	BlockScheduled *event.Event[*Block]
	// BlockDiscarded is triggered when a block is removed from the longest mana-scaled queue when the buffer is full.
	BlockDiscarded *event.Event[*Block]
	// BlockSkipped is triggered when a block is confirmed before it's scheduled, and is skipped by the scheduler.
	BlockSkipped *event.Event[*Block]
	Error        *event.Event[error]
}

func newEvents() (new *Events) {
	return &Events{
		BlockScheduled: event.New[*Block](),
		BlockDiscarded: event.New[*Block](),
		BlockSkipped:   event.New[*Block](),
		Error:          event.New[error](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
