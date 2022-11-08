package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

var (
	lastCommittedEpoch      = new(commitment.Commitment)
	lastCommittedEpochMutex sync.RWMutex

	acceptedBlksMutex sync.RWMutex
	numBlkOfEpoch     atomic.Int32
	currentEI         epoch.Index

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

func BlocksOfEpoch() (ei epoch.Index, num int32) {
	acceptedBlksMutex.Lock()
	defer acceptedBlksMutex.Unlock()

	return currentEI, numBlkOfEpoch.Load()
}

func updateBlkOfEpoch(ei epoch.Index, num int32) {
	acceptedBlksMutex.Lock()
	defer acceptedBlksMutex.Unlock()

	currentEI = ei
	numBlkOfEpoch.Store(num)
}

func RemovedBlocksOfEpoch() map[epoch.Index]int {
	orphanedBlkRemovedMutex.Lock()
	defer orphanedBlkRemovedMutex.Unlock()

	checkLimit()

	// copy the original map
	clone := make(map[epoch.Index]int)
	endEpoch := oldestEpoch + epoch.Index(orphanedBlkRemoved.Size())
	if endEpoch == 0 {
		endEpoch = epoch.IndexFromTime(time.Now())
	}

	for ei := oldestEpoch; ei <= endEpoch; ei++ {
		num, exists := orphanedBlkRemoved.Get(ei)
		if !exists {
			num = 0
			orphanedBlkRemoved.Set(ei, 0)
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

	checkLimit()
}

func checkLimit() {
	if orphanedBlkRemoved.Size() > maxEpochsPreserved {
		endEpoch := oldestEpoch + epoch.Index(numEpochsToRemove)
		for ei := oldestEpoch; ei <= endEpoch; ei++ {
			orphanedBlkRemoved.Delete(ei)
		}

		oldestEpoch += epoch.Index(numEpochsToRemove)
	}
}
