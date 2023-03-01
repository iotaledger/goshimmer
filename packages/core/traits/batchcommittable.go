package traits

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore"
)

// BatchCommittable is a trait that stores the latest commitment and metadata about batched state transitions.
type BatchCommittable interface {
	// BeginBatchedStateTransition starts a batched state transition to the given slot.
	BeginBatchedStateTransition(newSlot slot.Index) (currentSlot slot.Index, err error)

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
	// batchSlot is the slot that is currently being batched.
	batchSlot slot.Index

	// batchSlotMutex is used to synchronize access to batchSlot.
	batchSlotMutex sync.RWMutex

	// Committable is the underlying committable trait.
	Committable
}

// BeginBatchedStateTransition starts a batched state transition to the given slot.
func (b *batchCommittable) BeginBatchedStateTransition(newSlot slot.Index) (lastCommittedSlot slot.Index, err error) {
	b.batchSlotMutex.Lock()
	defer b.batchSlotMutex.Unlock()

	if newSlot != 0 && b.batchSlot != 0 {
		return 0, errors.Errorf("batch slot already set: previous=%d, new=%d", b.batchSlot, newSlot)
	}

	if lastCommittedSlot = b.LastCommittedSlot(); lastCommittedSlot == newSlot {
		return
	}

	if newSlot != 0 && (newSlot-lastCommittedSlot).Abs() > 1 {
		return 0, errors.New("batches can only be applied in order")
	}

	b.batchSlot = newSlot

	return
}

// FinalizeBatchedStateTransition finalizes the current batched state transition.
func (b *batchCommittable) FinalizeBatchedStateTransition() {
	b.batchSlotMutex.Lock()
	defer b.batchSlotMutex.Unlock()

	b.SetLastCommittedSlot(b.batchSlot)
	b.batchSlot = 0
}

// BatchedStateTransitionStarted returns true if a batched state transition is currently in progress.
func (b *batchCommittable) BatchedStateTransitionStarted() bool {
	b.batchSlotMutex.RLock()
	defer b.batchSlotMutex.RUnlock()

	return b.batchSlot != 0
}
