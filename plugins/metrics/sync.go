package metrics

import (
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"go.uber.org/atomic"
)

var (
	isSynced atomic.Bool
)

func measureSynced() {
	s := messagelayer.Tangle().Synced()
	metrics.Events().Synced.Trigger(s)
}

// Synced returns if the node is synced.
func Synced() bool {
	return isSynced.Load()
}
