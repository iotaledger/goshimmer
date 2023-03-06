package scheduler

import (
	"github.com/iotaledger/hive.go/runtime/event"
)

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Scheduler.
type Events struct {
	// BlockScheduled is triggered when a block is ready to be scheduled.
	BlockScheduled *event.Event1[*Block]
	// BlockSubmitted is triggered when a block is ready to be scheduled.
	BlockSubmitted *event.Event1[*Block]
	// BlockDropped is triggered when a block is removed from the longest mana-scaled queue when the buffer is full.
	BlockDropped *event.Event1[*Block]
	// BlockSkipped is triggered when a block is confirmed before it's scheduled, and is skipped by the scheduler.
	BlockSkipped *event.Event1[*Block]
	Error        *event.Event1[error]

	event.Group[Events, *Events]
}

var NewEvents = event.CreateGroupConstructor(func() (newEvents *Events) {
	return &Events{
		BlockScheduled: event.New1[*Block](),
		BlockSubmitted: event.New1[*Block](),
		BlockDropped:   event.New1[*Block](),
		BlockSkipped:   event.New1[*Block](),
		Error:          event.New1[error](),
	}
})

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
