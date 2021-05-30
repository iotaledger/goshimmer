package metrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var isTangleTimeSynced atomic.Bool

func measureSynced() {
	tts := messagelayer.Tangle().TimeManager.Synced()
	metrics.Events().TangleTimeSynced.Trigger(tts)
}

// TangleTimeSynced returns if the node is synced based on tangle time.
func TangleTimeSynced() bool {
	return isTangleTimeSynced.Load()
}
