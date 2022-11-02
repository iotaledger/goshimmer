package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

var (
	lastCommittedEpoch      = new(commitment.Commitment)
	lastCommittedEpochMutex sync.RWMutex
)

var onEpochCommitted = event.NewClosure(func(commitment *commitment.Commitment) {
	lastCommittedEpochMutex.Lock()
	defer lastCommittedEpochMutex.Unlock()

	lastCommittedEpoch = commitment
})

// LastCommittedEpoch returns the last committed epoch.
func LastCommittedEpoch() *commitment.Commitment {
	lastCommittedEpochMutex.RLock()
	defer lastCommittedEpochMutex.RUnlock()
	return lastCommittedEpoch
}
