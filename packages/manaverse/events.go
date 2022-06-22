package manaverse

import (
	"time"

	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

// region SchedulerEvents //////////////////////////////////////////////////////////////////////////////////////////////

type SchedulerEvents struct {
	BlockQueued              *event.Event[*SchedulerBlockEvent]
	BlockScheduled           *event.Event[*SchedulerBlockEvent]
	BlockDropped             *event.Event[*SchedulerBlockEvent]
	BucketProcessingStarted  *event.Event[*SchedulerBucketEvent]
	BucketProcessingFinished *event.Event[*SchedulerBucketEvent]
}

func newSchedulerEvents() (newInstance *SchedulerEvents) {
	return &SchedulerEvents{
		BlockQueued:              event.New[*SchedulerBlockEvent](),
		BlockScheduled:           event.New[*SchedulerBlockEvent](),
		BlockDropped:             event.New[*SchedulerBlockEvent](),
		BucketProcessingStarted:  event.New[*SchedulerBucketEvent](),
		BucketProcessingFinished: event.New[*SchedulerBucketEvent](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerBlockEvent //////////////////////////////////////////////////////////////////////////////////////////

type SchedulerBlockEvent struct {
	Block  *tangle.Message
	Bucket int64
	Time   time.Time
}

func newSchedulerBlockEvent(block *tangle.Message, bucket int64, time time.Time) (newInstance *SchedulerBlockEvent) {
	return &SchedulerBlockEvent{
		Block:  block,
		Bucket: bucket,
		Time:   time,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerBucketEvent /////////////////////////////////////////////////////////////////////////////////////////

type SchedulerBucketEvent struct {
	Bucket int64
	Time   time.Time
}

func newSchedulerBucketEvent(bucket int64, time time.Time) (newInstance *SchedulerBucketEvent) {
	return &SchedulerBucketEvent{
		Bucket: bucket,
		Time:   time,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
