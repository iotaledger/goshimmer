package remotelogmetrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	isTangleTimeSynced atomic.Bool
)

func checkSynced() {
	oldTangleTimeSynced := isTangleTimeSynced.Load()
	tts := messagelayer.Tangle().TimeManager.Synced()
	if oldTangleTimeSynced != tts {
		var myID string
		if local.GetInstance() != nil {
			myID = local.GetInstance().ID().String()
		}
		syncStatusChangedEvent := remotelogmetrics.SyncStatusChangedEvent{
			Type:                     "sync",
			NodeID:                   myID,
			Time:                     clock.SyncedTime(),
			LastConfirmedMessageTime: messagelayer.Tangle().TimeManager.Time(),
			CurrentStatus:            tts,
			PreviousStatus:           oldTangleTimeSynced,
		}
		remotelogmetrics.Events().TangleTimeSyncChanged.Trigger(syncStatusChangedEvent)
	}
}
