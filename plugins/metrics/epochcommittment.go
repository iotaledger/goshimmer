package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

var (
	lastCommittedEpoch      = new(commitment.Commitment)
	lastCommittedEpochMutex sync.RWMutex

	orphanedBlkRemovedMutex sync.RWMutex
	oldestEpoch             = epoch.Index(0)
	maxEpochsPreserved      = 20
	numEpochsToRemove       = maxEpochsPreserved / 2
	orphanedBlkRemoved      = shrinkingmap.New[epoch.Index, int]()
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

func RemovedBlocksOfEpoch() map[epoch.Index]int {
	orphanedBlkRemovedMutex.RLock()
	defer orphanedBlkRemovedMutex.RUnlock()

	// copy the original map
	clone := make(map[epoch.Index]int)
	endEpoch := oldestEpoch + epoch.Index(orphanedBlkRemoved.Size())
	for ei := oldestEpoch; ei <= endEpoch; ei++ {
		num, exists := orphanedBlkRemoved.Get(ei)
		if !exists {
			num = 0
		}
		clone[ei] = num
	}

	return clone
}

func increaseRemovedBlockCounter(blkID models.BlockID) {
	orphanedBlkRemovedMutex.Lock()
	defer orphanedBlkRemovedMutex.Unlock()

	if count, exists := orphanedBlkRemoved.Get(blkID.EpochIndex); exists {
		count++
		orphanedBlkRemoved.Set(blkID.EpochIndex, count)
	} else {
		orphanedBlkRemoved.Set(blkID.EpochIndex, 1)
	}

	if orphanedBlkRemoved.Size() > maxEpochsPreserved {
		endEpoch := oldestEpoch + epoch.Index(numEpochsToRemove)
		for ei := oldestEpoch; ei <= endEpoch; ei++ {
			orphanedBlkRemoved.Delete(ei)
		}

		oldestEpoch += epoch.Index(numEpochsToRemove)
	}
}
