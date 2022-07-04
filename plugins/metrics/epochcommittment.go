package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/notarization"
)

var (
	lastCommittedEpoch      = epoch.NewECRecord(epoch.Index(0))
	lastCommittedEpochMutex sync.RWMutex
)

var (
	onEpochCommitted = event.NewClosure(func(event *notarization.EpochCommittedEvent) {
		lastCommittedEpochMutex.Lock()
		defer lastCommittedEpochMutex.Unlock()
		lastCommittedEpoch = event.ECRecord
	})
)

// LastCommittedEpoch returns the last committed epoch.
func LastCommittedEpoch() *epoch.ECRecord {
	lastCommittedEpochMutex.RLock()
	defer lastCommittedEpochMutex.RUnlock()
	return lastCommittedEpoch
}
