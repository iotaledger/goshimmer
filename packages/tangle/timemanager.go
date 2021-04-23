package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/events"
)

type TimeManager struct {
	tangle *Tangle

	lastConfirmedTime      time.Time
	lastConfirmedTimeMutex sync.RWMutex
}

func NewTimeManager(tangle *Tangle) *TimeManager {
	return &TimeManager{
		tangle: tangle,
	}
	// TODO: set default value for lastConfirmedTime when starting up
}

func (t *TimeManager) Setup() {
	// TODO: maybe it is better to attach to an event for each message being confirmed.
	//  However, we're confirming a marker and walking in its past cone, hence it should have the most recent timestamp.
	t.tangle.ApprovalWeightManager.Events.MarkerConfirmation.Attach(events.NewClosure(t.updateTime))
}

func (t *TimeManager) Time() time.Time {
	t.lastConfirmedTimeMutex.RLock()
	defer t.lastConfirmedTimeMutex.RUnlock()

	return t.lastConfirmedTime
}

func (t *TimeManager) Synced() bool {
	t.lastConfirmedTimeMutex.RLock()
	defer t.lastConfirmedTimeMutex.RUnlock()

	return clock.SyncedTime().Sub(t.lastConfirmedTime) < t.tangle.Options.SyncTimeWindow
}

func (t *TimeManager) updateTime(marker markers.Marker, newLevel int, transition events.ThresholdEventTransition) {
	if transition != events.ThresholdLevelIncreased {
		return
	}
	// get message ID of marker
	messageID := t.tangle.Booker.MarkersManager.MessageID(&marker)

	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		t.lastConfirmedTimeMutex.Lock()
		defer t.lastConfirmedTimeMutex.Unlock()

		if t.lastConfirmedTime.Before(message.IssuingTime()) {
			t.lastConfirmedTime = message.IssuingTime()
		}
	})
}
