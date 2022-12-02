package traits

import (
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type BatchCommittable interface {
	BeginBatchedStateTransition(newEpoch epoch.Index) (currentEpoch epoch.Index, err error)
	BatchedStateTransitionStarted() (wasStarted bool)
	FinalizeBatchedStateTransition()

	Committable
}

func NewBatchCommittable() (commitmentState BatchCommittable) {
	return &batchCommittable{
		Committable: NewCommittable(),
	}
}

type batchCommittable struct {
	batchEpoch      epoch.Index
	batchEpochMutex sync.RWMutex

	Committable
}

func (u *batchCommittable) BeginBatchedStateTransition(newEpoch epoch.Index) (lastCommittedEpoch epoch.Index, err error) {
	u.batchEpochMutex.Lock()
	defer u.batchEpochMutex.Unlock()

	if newEpoch != 0 && u.batchEpoch != 0 {
		return 0, errors.New("batch epoch already set")
	}

	if lastCommittedEpoch = u.LastCommittedEpoch(); lastCommittedEpoch == newEpoch {
		return
	}

	if newEpoch != 0 && (newEpoch-lastCommittedEpoch).Abs() > 1 {
		return 0, errors.New("batches can only be applied in order")
	}

	u.batchEpoch = newEpoch

	return
}

func (u *batchCommittable) FinalizeBatchedStateTransition() {
	u.batchEpochMutex.Lock()
	defer u.batchEpochMutex.Unlock()

	u.SetLastCommittedEpoch(u.batchEpoch)
	u.batchEpoch = 0
}

func (u *batchCommittable) BatchedStateTransitionStarted() bool {
	u.batchEpochMutex.RLock()
	defer u.batchEpochMutex.RUnlock()

	return u.batchEpoch != 0
}
