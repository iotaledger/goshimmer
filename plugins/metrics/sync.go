package metrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var (
	isSyncBeaconSynced atomic.Bool
	isTangleTimeSynced atomic.Bool
)

func measureSynced() {
	sbs := messagelayer.Tangle().Synced()
	metrics.Events().SyncBeaconSynced.Trigger(sbs)

	tts := messagelayer.Tangle().TimeManager.Synced()
	metrics.Events().TangleTimeSynced.Trigger(tts)
}

// TangleTimeSynced returns if the node is synced based on tangle time.
func TangleTimeSynced() bool {
	return isTangleTimeSynced.Load()
}

// SyncBeaconSynced returns if the node is synced based on sync beacon.
func SyncBeaconSynced() bool {
	return isSyncBeaconSynced.Load()
}
