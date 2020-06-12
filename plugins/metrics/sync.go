package metrics

import (
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/sync"
	"github.com/iotaledger/hive.go/syncutils"
)

var (
	isSynced bool
	syncLock syncutils.RWMutex
)

func measureSynced() {
	s := sync.Synced()
	metrics.Events().Synced.Trigger(s)
}

// Synced returns if the node is synced.
func Synced() bool {
	syncLock.RLock()
	defer syncLock.RUnlock()
	return isSynced
}
