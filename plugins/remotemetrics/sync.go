package remotemetrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/plugins/remotelog"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var isTangleTimeSynced atomic.Bool

func checkSynced() {
	oldTangleTimeSynced := isTangleTimeSynced.Load()
	tts := messagelayer.Tangle().TimeManager.Synced()
	if oldTangleTimeSynced != tts {
		var myID string
		if local.GetInstance() != nil {
			myID = local.GetInstance().ID().String()
		}
		syncStatusChangedEvent := remotemetrics.SyncStatusChangedEvent{
			Type:                     "sync",
			NodeID:                   myID,
			MetricsLevel:             Parameters.MetricsLevel,
			Time:                     clock.SyncedTime(),
			LastConfirmedMessageTime: messagelayer.Tangle().TimeManager.Time(),
			CurrentStatus:            tts,
			PreviousStatus:           oldTangleTimeSynced,
		}
		remotemetrics.Events().TangleTimeSyncChanged.Trigger(syncStatusChangedEvent)
	}
}

func sendSyncStatusChangedEvent(syncUpdate remotemetrics.SyncStatusChangedEvent) {
	err := remotelog.RemoteLogger().Send(syncUpdate)
	if err != nil {
		plugin.Logger().Errorw("Failed to send sync status changed record on sync change event.", "err", err)
	}
}
