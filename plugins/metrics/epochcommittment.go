package metrics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

var (
	lastCommitment          = new(commitment.Commitment)
	lastCommittedEpochMutex sync.RWMutex

	acceptedBlocksCount       int32
	acceptedTransactionsCount int32
	activeValidatorsCount     int32
	currentEI                 epoch.Index
	epochCountsMutex          sync.RWMutex

	numberOfSeenOtherCommitments = atomic.Int64{}

	missingCommitments         = atomic.Int64{}
	missingCommitmentsReceived = atomic.Int64{}

	oldestEpoch             = epoch.Index(0)
	maxEpochsPreserved      = 20
	numEpochsToRemove       = maxEpochsPreserved / 2
	orphanedBlkRemoved      = shrinkingmap.New[epoch.Index, int]()
	orphanedBlkRemovedMutex sync.RWMutex
)

var onEpochCommitted = event.NewClosure(func(details *notarization.EpochCommittedDetails) {
	lastCommittedEpochMutex.Lock()
	defer lastCommittedEpochMutex.Unlock()

	lastCommitment = details.Commitment
})

var onForkDetected = event.NewClosure(func(chain *chainmanager.Chain) {
	numberOfSeenOtherCommitments.Add(1)
})

var onCommitmentMissing = event.NewClosure(func(commitment commitment.ID) {
	missingCommitments.Add(1)
})

var onMissingCommitmentReceived = event.NewClosure(func(commitment commitment.ID) {
	missingCommitmentsReceived.Add(1)
})

// LastCommittedEpoch returns the last committed epoch.
func LastCommittedEpoch() *commitment.Commitment {
	lastCommittedEpochMutex.RLock()
	defer lastCommittedEpochMutex.RUnlock()

	return lastCommitment
}

// NumberOfSeenOtherCommitments returns the number of commitments different from ours that were observed by the node.
func NumberOfSeenOtherCommitments() int64 {
	return numberOfSeenOtherCommitments.Load()
}

// MissingCommitmentsRequested returns the number of requested missing commitments.
func MissingCommitmentsRequested() int64 {
	return missingCommitments.Load()
}

// MissingCommitmentsReceived returns the number of received missing commitments.
func MissingCommitmentsReceived() int64 {
	return missingCommitmentsReceived.Load()
}

// BlocksOfEpoch returns the number of blocks in the current epoch.
func BlocksOfEpoch() (ei epoch.Index, accBlocks, accTxs, activeValidators int32) {
	epochCountsMutex.Lock()
	defer epochCountsMutex.Unlock()

	return currentEI, acceptedBlocksCount, acceptedTransactionsCount, activeValidatorsCount
}

func updateBlkOfEpoch(ei epoch.Index, accBlocks, accTxs, activeValidators int32) {
	epochCountsMutex.Lock()
	defer epochCountsMutex.Unlock()

	currentEI = ei
	acceptedBlocksCount = accBlocks
	acceptedTransactionsCount = accTxs
	activeValidatorsCount = activeValidators
}

// RemovedBlocksOfEpoch returns the number of orphaned blocks in the given epoch.
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
