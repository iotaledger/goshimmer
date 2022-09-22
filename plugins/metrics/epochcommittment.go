package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
)

var (
	lastCommittedEpoch      = chainmanager.NewCommitment(commitment.NewMerkleRoot([]byte{}))
	lastCommittedEpochMutex sync.RWMutex
)

var onEpochCommitted = event.NewClosure(func(event *notarization.EpochCommittableEvent) {
	lastCommittedEpochMutex.Lock()
	defer lastCommittedEpochMutex.Unlock()
	lastCommittedEpoch = event.ECRecord
})

// LastCommittedEpoch returns the last committed epoch.
func LastCommittedEpoch() *chainmanager.Commitment {
	lastCommittedEpochMutex.RLock()
	defer lastCommittedEpochMutex.RUnlock()
	return lastCommittedEpoch
}
