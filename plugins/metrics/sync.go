package metrics

import (
	"go.uber.org/atomic"
)

var isTangleTimeSynced atomic.Bool

func measureSynced() {
	tts := deps.Tangle.TimeManager.Bootstrapped()
	isTangleTimeSynced.Store(tts)
}

// TangleTimeSynced returns if the node is synced based on tangle time.
func TangleTimeSynced() bool {
	return isTangleTimeSynced.Load()
}
