package remotelogmetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/drng"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

type dRNGMetricsLogger struct {
	Type              string    `json:"type" bson:"type"`
	InstanceID        uint32    `json:"instanceID" bson:"instanceID"`
	Round             uint64    `json:"round" bson:"round"`
	IssuedTimestamp   time.Time `json:"issuedTimestamp" bson:"issuedTimestamp"`
	ReceivedTimestamp time.Time `json:"receivedTimestamp" bson:"receivedTimestamp"`
}

func newDRNGMetricsLogger() *dRNGMetricsLogger {
	return &dRNGMetricsLogger{}
}

func (ml *dRNGMetricsLogger) onRandomnessReceived(state *drng.State) {
	record := &dRNGMetricsLogger{
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
