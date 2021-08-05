package remotemetrics

import (
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

func onRandomnessReceived(state *drng.State) {
	if !messagelayer.Tangle().Synced() {
		return
	}

	var nodeID string
	if local.GetInstance() != nil {
		nodeID = local.GetInstance().ID().String()
	}

	record := &remotemetrics.DRNGMetrics{
		Type:              "drng",
		NodeID:            nodeID,
		MetricsLevel:      Parameters.MetricsLevel,
		InstanceID:        state.Committee().InstanceID,
		Round:             state.Randomness().Round,
		IssuedTimestamp:   state.Randomness().Timestamp,
		ReceivedTimestamp: clock.SyncedTime(),
		DeltaReceived:     clock.Since(state.Randomness().Timestamp).Nanoseconds(),
	}

	if err := remotelog.RemoteLogger().Send(record); err != nil {
		plugin.Logger().Errorw("Failed to send Randomness record", "err", err)
	}
}
