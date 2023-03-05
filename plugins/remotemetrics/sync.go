package remotemetrics

import (
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
)

var isTangleTimeSynced atomic.Bool

func checkSynced() {
	oldTangleTimeSynced := isTangleTimeSynced.Load()
	tts := deps.Protocol.Engine().IsSynced()
	if oldTangleTimeSynced != tts {
		var myID string
		if deps.Local != nil {
			myID = deps.Local.ID().String()
		}
		syncStatusChangedEvent := &remotemetrics.TangleTimeSyncChangedEvent{
			Type:           "sync",
			NodeID:         myID,
			MetricsLevel:   Parameters.MetricsLevel,
			Time:           time.Now(),
			ATT:            deps.Protocol.Engine().Clock.AcceptanceTime().Get(),
			RATT:           deps.Protocol.Engine().Clock.AcceptanceTime().Now(),
			CTT:            deps.Protocol.Engine().Clock.ConfirmationTime().Get(),
			RCTT:           deps.Protocol.Engine().Clock.ConfirmationTime().Now(),
			CurrentStatus:  tts,
			PreviousStatus: oldTangleTimeSynced,
		}
		remotemetrics.Events.TangleTimeSyncChanged.Trigger(syncStatusChangedEvent)
	}
}

func sendSyncStatusChangedEvent(syncUpdate *remotemetrics.TangleTimeSyncChangedEvent) {
	_ = deps.RemoteLogger.Send(syncUpdate)
}
