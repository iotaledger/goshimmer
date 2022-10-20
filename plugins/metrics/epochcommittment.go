package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
)

var (
	lastCommittedEpoch      = chainmanager.NewCommitment(commitment.NewID(0, []byte{}))
	lastCommittedEpochMutex sync.RWMutex
)

var onEpochCommitted = event.NewClosure(func(event *notarization.EpochCommittedEvent) {
	lastCommittedEpochMutex.Lock()
	defer lastCommittedEpochMutex.Unlock()
	lastCommittedEpoch = event.Commitment
})

// LastCommittedEpoch returns the last committed epoch.
func LastCommittedEpoch() *chainmanager.Commitment {
	lastCommittedEpochMutex.RLock()
	defer lastCommittedEpochMutex.RUnlock()
	return lastCommittedEpoch
}
