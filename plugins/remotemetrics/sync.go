package remotemetrics

import (
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
)

var isTangleTimeSynced atomic.Bool

func checkSynced() {
	oldTangleTimeSynced := isTangleTimeSynced.Load()
	tts := deps.Protocol.Instance().Engine.IsSynced()
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
			ATT:            deps.Protocol.Instance().Engine.Clock.AcceptedTime(),
			RATT:           deps.Protocol.Instance().Engine.Clock.RelativeAcceptedTime(),
			CTT:            deps.Protocol.Instance().Engine.Clock.ConfirmedTime(),
			RCTT:           deps.Protocol.Instance().Engine.Clock.RelativeConfirmedTime(),
			CurrentStatus:  tts,
			PreviousStatus: oldTangleTimeSynced,
		}
		remotemetrics.Events.TangleTimeSyncChanged.Trigger(syncStatusChangedEvent)
	}
}

func sendSyncStatusChangedEvent(syncUpdate *remotemetrics.TangleTimeSyncChangedEvent) {
	_ = deps.RemoteLogger.Send(syncUpdate)
}
