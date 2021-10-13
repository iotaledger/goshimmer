package metrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/metrics"
)

var isTangleTimeSynced atomic.Bool

func measureSynced() {
	tts := deps.Tangle.TimeManager.Synced()
	metrics.Events().TangleTimeSynced.Trigger(tts)
}

// TangleTimeSynced returns if the node is synced based on tangle time.
func TangleTimeSynced() bool {
	return isTangleTimeSynced.Load()
}
