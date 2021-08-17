package remotelogmetrics

import (
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
)

func onRandomnessReceived(state *drng.State) {
	if !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.ID().String()
	}

	record := &remotelogmetrics.DRNGMetrics{
		Type:              "drng",
		NodeID:            nodeID,
		InstanceID:        state.Committee().InstanceID,
		Round:             state.Randomness().Round,
		IssuedTimestamp:   state.Randomness().Timestamp,
		ReceivedTimestamp: clock.SyncedTime(),
		DeltaReceived:     clock.Since(state.Randomness().Timestamp).Nanoseconds(),
	}

	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send Randomness record", "err", err)
	}
}
