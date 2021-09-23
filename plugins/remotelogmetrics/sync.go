package remotelogmetrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
)

var isTangleTimeSynced atomic.Bool

func checkSynced() {
	oldTangleTimeSynced := isTangleTimeSynced.Load()
	tts := deps.Tangle.TimeManager.Synced()
	if oldTangleTimeSynced != tts {
		var myID string
		if deps.Local != nil {
			myID = deps.Local.ID().String()
		}
		syncStatusChangedEvent := remotelogmetrics.SyncStatusChangedEvent{
			Type:                     "sync",
			NodeID:                   myID,
			Time:                     clock.SyncedTime(),
			LastConfirmedMessageTime: deps.Tangle.TimeManager.Time(),
			CurrentStatus:            tts,
			PreviousStatus:           oldTangleTimeSynced,
		}
		remotelogmetrics.Events().TangleTimeSyncChanged.Trigger(syncStatusChangedEvent)
	}
}
