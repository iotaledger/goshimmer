package traits

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/hive.go/core/kvstore"
)

// BatchCommittable is a trait that stores the latest commitment and metadata about batched state transitions.
type BatchCommittable interface {
	// BeginBatchedStateTransition starts a batched state transition to the given epoch.
	BeginBatchedStateTransition(newEpoch epoch.Index) (currentEpoch epoch.Index, err error)

	// FinalizeBatchedStateTransition finalizes the current batched state transition.
	FinalizeBatchedStateTransition()

	// BatchedStateTransitionStarted returns true if a batched state transition is currently in progress.
	BatchedStateTransitionStarted() (wasStarted bool)

	// Committable is the underlying committable trait.
	Committable
}

// NewBatchCommittable creates a new BatchCommittable trait.
func NewBatchCommittable(store kvstore.KVStore, keyBytes ...byte) (newBatchCommittable BatchCommittable) {
	return &batchCommittable{
		Committable: NewCommittable(store, keyBytes...),
	}
}

// batchCommittable is the implementation of the BatchCommittable trait.
type batchCommittable struct {
	// batchEpoch is the epoch that is currently being batched.
	batchEpoch epoch.Index

	// batchEpochMutex is used to synchronize access to batchEpoch.
	batchEpochMutex sync.RWMutex

	// Committable is the underlying committable trait.
	Committable
}

// BeginBatchedStateTransition starts a batched state transition to the given epoch.
func (b *batchCommittable) BeginBatchedStateTransition(newEpoch epoch.Index) (lastCommittedEpoch epoch.Index, err error) {
	b.batchEpochMutex.Lock()
	defer b.batchEpochMutex.Unlock()

	if newEpoch != 0 && b.batchEpoch != 0 {
		return 0, errors.Errorf("batch epoch already set: previous=%d, new=%d", b.batchEpoch, newEpoch)
	}

	if lastCommittedEpoch = b.LastCommittedEpoch(); lastCommittedEpoch == newEpoch {
		return
	}

	if newEpoch != 0 && (newEpoch-lastCommittedEpoch).Abs() > 1 {
		return 0, errors.New("batches can only be applied in order")
	}

	b.batchEpoch = newEpoch

	return
}

// FinalizeBatchedStateTransition finalizes the current batched state transition.
func (b *batchCommittable) FinalizeBatchedStateTransition() {
	b.batchEpochMutex.Lock()
	defer b.batchEpochMutex.Unlock()

	b.SetLastCommittedEpoch(b.batchEpoch)
	b.batchEpoch = 0
}

// BatchedStateTransitionStarted returns true if a batched state transition is currently in progress.
func (b *batchCommittable) BatchedStateTransitionStarted() bool {
	b.batchEpochMutex.RLock()
	defer b.batchEpochMutex.RUnlock()

	return b.batchEpoch != 0
}
