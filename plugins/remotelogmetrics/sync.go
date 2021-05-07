package remotelogmetrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"

	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	isSyncBeaconSynced atomic.Bool
	isTangleTimeSynced atomic.Bool
)

func checkSynced() {
	oldSyncBeaconSynced := isSyncBeaconSynced.Load()
	sbs := messagelayer.Tangle().Synced()
	if oldSyncBeaconSynced != sbs {
		syncStatusChangedEvent := remotelogmetrics.SyncStatusChangedEvent{
			Type:           "sync",
			NodeID:         local.GetInstance().ID().String(),
			Time:           clock.SyncedTime(),
			CurrentStatus:  sbs,
			PreviousStatus: oldSyncBeaconSynced,
			SyncType:       "syncbeacon",
		}
		remotelogmetrics.Events().SyncBeaconSyncChanged.Trigger(syncStatusChangedEvent)
	}

	oldTangleTimeSynced := isTangleTimeSynced.Load()
	tts := messagelayer.Tangle().TimeManager.Synced()
	if oldTangleTimeSynced != tts {
		syncStatusChangedEvent := remotelogmetrics.SyncStatusChangedEvent{
			Type:                     "sync",
			NodeID:                   local.GetInstance().ID().String(),
			Time:                     clock.SyncedTime(),
			LastConfirmedMessageTime: messagelayer.Tangle().TimeManager.Time(),
			CurrentStatus:            tts,
			PreviousStatus:           oldTangleTimeSynced,
			SyncType:                 "tangletime",
		}
		remotelogmetrics.Events().TangleTimeSyncChanged.Trigger(syncStatusChangedEvent)
	}
}
