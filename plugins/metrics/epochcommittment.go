package metrics

import (
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
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
	lastCommitment          = new(commitment.Commitment)
	lastCommittedEpochMutex sync.RWMutex

	acceptedBlocksCount       atomic.Int32
	acceptedTransactionsCount atomic.Int32
	activeValidatorsCount     atomic.Int32
	currentEI                 epoch.Index
	epochCountsMutex          sync.RWMutex

	numberOfSeenOtherCommitments = 0
	seenOtherCommitmentsMutex    sync.RWMutex

	missingCommitments         = 0
	missingCommitmentsReceived = 0
	commitmentsMutex           sync.RWMutex

	orphanedBlkRemovedMutex sync.RWMutex
	oldestEpoch             = epoch.Index(0)
	maxEpochsPreserved      = 20
	numEpochsToRemove       = maxEpochsPreserved / 2
	orphanedBlkRemoved      = shrinkingmap.New[epoch.Index, int]()
)

var onEpochCommitted = event.NewClosure(func(commitment *commitment.Commitment) {
	lastCommittedEpochMutex.Lock()
	defer lastCommittedEpochMutex.Unlock()

	lastCommitment = commitment
})

var onForkDetected = event.NewClosure(func(chain *chainmanager.Chain) {
	seenOtherCommitmentsMutex.Lock()
	defer seenOtherCommitmentsMutex.Unlock()

	numberOfSeenOtherCommitments++
})

var onCommitmentMissing = event.NewClosure(func(commitment commitment.ID) {
	commitmentsMutex.Lock()
	defer commitmentsMutex.Unlock()

	missingCommitments++
})

var onMissingCommitmentReceived = event.NewClosure(func(commitment commitment.ID) {
	commitmentsMutex.Lock()
	defer commitmentsMutex.Unlock()

	missingCommitmentsReceived++
})

// LastCommittedEpoch returns the last committed epoch.
func LastCommittedEpoch() *commitment.Commitment {
	lastCommittedEpochMutex.RLock()
	defer lastCommittedEpochMutex.RUnlock()

	return lastCommitment
}

// NumberOfSeenOtherCommitments returns the number of commitments different from ours that were observed by the node.
func NumberOfSeenOtherCommitments() int {
	seenOtherCommitmentsMutex.RLock()
	defer seenOtherCommitmentsMutex.RUnlock()

	return numberOfSeenOtherCommitments
}

// MissingCommitmentsRequested returns the number of requested missing commitments.
func MissingCommitmentsRequested() int {
	commitmentsMutex.RLock()
	defer commitmentsMutex.RUnlock()

	return missingCommitments
}

// MissingCommitmentsReceived returns the number of received missing commitments.
func MissingCommitmentsReceived() int {
	commitmentsMutex.RLock()
	defer commitmentsMutex.RUnlock()

	return missingCommitmentsReceived
}

// BlocksOfEpoch returns the number of blocks in the current epoch.
func BlocksOfEpoch() (ei epoch.Index, accBlocks int32, accTxs int32, activeValidators int32) {
	epochCountsMutex.Lock()
	defer epochCountsMutex.Unlock()

	return currentEI, acceptedBlocksCount.Load(), acceptedTransactionsCount.Load(), activeValidatorsCount.Load()
}

func updateBlkOfEpoch(ei epoch.Index, accBlocks int32, accTxs int32, activeValidators int32) {
	epochCountsMutex.Lock()
	defer epochCountsMutex.Unlock()

	currentEI = ei
	acceptedBlocksCount.Store(accBlocks)
	acceptedTransactionsCount.Store(accTxs)
	activeValidatorsCount.Store(activeValidators)
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
