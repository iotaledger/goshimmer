package metrics

import (
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var isSynced atomic.Bool

func measureSynced() {
	s := messagelayer.Tangle().Synced()
	metrics.Events().Synced.Trigger(s)
}

// Synced returns if the node is synced.
func Synced() bool {
	return isSynced.Load()
}
