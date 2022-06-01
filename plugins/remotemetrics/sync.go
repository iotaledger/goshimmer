package remotemetrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
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
		syncStatusChangedEvent := &remotemetrics.TangleTimeSyncChangedEvent{
			Type:           "sync",
			NodeID:         myID,
			MetricsLevel:   Parameters.MetricsLevel,
			Time:           clock.SyncedTime(),
			CTT:            deps.Tangle.TimeManager.CTT(),
			RCTT:           deps.Tangle.TimeManager.RCTT(),
			FTT:            deps.Tangle.TimeManager.FTT(),
			RFTT:           deps.Tangle.TimeManager.RFTT(),
			CurrentStatus:  tts,
			PreviousStatus: oldTangleTimeSynced,
		}
		remotemetrics.Events.TangleTimeSyncChanged.Trigger(syncStatusChangedEvent)
	}
}

func sendSyncStatusChangedEvent(syncUpdate *remotemetrics.TangleTimeSyncChangedEvent) {
	_ = deps.RemoteLogger.Send(syncUpdate)
}
