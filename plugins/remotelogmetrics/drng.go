package remotelogmetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

type dRNGMetricsLogger struct {
	InstanceID        uint32    `json:"instanceID"`
	Round             uint64    `json:"round"`
	IssuedTimestamp   time.Time `json:"issuedTimestamp"`
	ReceivedTimestamp time.Time `json:"receivedTimestamp"`
}

func (ml *dRNGMetricsLogger) onRandomnessReceived(state *drng.State) {
	record := &dRNGMetricsLogger{
		InstanceID:        state.Committee().InstanceID,
		Round:             state.Randomness().Round,
		IssuedTimestamp:   state.Randomness().Timestamp,
		ReceivedTimestamp: clock.SyncedTime(),
	}

	if err := remotelog.RemoteLogger().Send(record); err != nil {
		plugin.Logger().Errorw("Failed to send Randomness record", "err", err)
	}
}
