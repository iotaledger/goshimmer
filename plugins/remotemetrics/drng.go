package remotemetrics

import (
	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
)

func onRandomnessReceived(state *drng.State) {
	if !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.ID().String()
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

	_ = deps.RemoteLogger.Send(record)
}
