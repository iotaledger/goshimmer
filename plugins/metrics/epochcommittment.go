package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

var (
	lastCommittedEpoch      = epoch.NewECRecord(epoch.Index(0))
	lastCommittedEpochMutex sync.RWMutex
)

var onEpochCommitted = event.NewClosure(func(event *notarization.EpochCommittableEvent) {
	lastCommittedEpochMutex.Lock()
	defer lastCommittedEpochMutex.Unlock()
	lastCommittedEpoch = event.ECRecord
})

// LastCommittedEpoch returns the last committed epoch.
func LastCommittedEpoch() *epoch.ECRecord {
	lastCommittedEpochMutex.RLock()
	defer lastCommittedEpochMutex.RUnlock()
	return lastCommittedEpoch
}
