package metrics

import (
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/sync"
	"go.uber.org/atomic"
)

var (
	isSynced atomic.Bool
)

func measureSynced() {
	s := sync.Synced()
	metrics.Events().Synced.Trigger(s)
}

// Synced returns if the node is synced.
func Synced() bool {
	return isSynced.Load()
}
