package remotelogmetrics

import (
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

func onRandomnessReceived(state *drng.State) {
	if !messagelayer.Tangle().Synced() {
		return
	}

	record := &remotelogmetrics.DRNGMetrics{
		Type:              "drng",
		InstanceID:        state.Committee().InstanceID,
		Round:             state.Randomness().Round,
		IssuedTimestamp:   state.Randomness().Timestamp,
		ReceivedTimestamp: clock.SyncedTime(),
	}

	if err := remotelog.RemoteLogger().Send(record); err != nil {
		plugin.Logger().Errorw("Failed to send Randomness record", "err", err)
	}
}
