package ledgerstate

import (
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type CommitmentState struct {
	lastCommittedEpochIndex      epoch.Index
	lastCommittedEpochIndexMutex sync.RWMutex
	batchEpoch                   epoch.Index
	batchEpochMutex              sync.RWMutex
}

func NewCommitmentState() (commitmentState *CommitmentState) {
	return &CommitmentState{}
}

func (u *CommitmentState) BeginBatchedStateTransition(newEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	u.batchEpochMutex.Lock()
	defer u.batchEpochMutex.Unlock()

	if newEpoch != 0 && u.batchEpoch != 0 {
		return 0, errors.New("batch epoch already set")
	}

	if currentEpoch = u.LastCommittedEpoch(); currentEpoch == newEpoch {
		return
	}

	if newEpoch != 0 && (newEpoch-currentEpoch).Abs() > 1 {
		return 0, errors.New("batches can only be applied in order")
	}

	u.batchEpoch = newEpoch

	return
}

func (u *CommitmentState) FinalizeBatchedStateTransition() {
	u.batchEpochMutex.Lock()
	defer u.batchEpochMutex.Unlock()

	u.SetLastCommittedEpoch(u.batchEpoch)
	u.batchEpoch = 0
}

func (u *CommitmentState) LastCommittedEpoch() epoch.Index {
	u.lastCommittedEpochIndexMutex.RLock()
	defer u.lastCommittedEpochIndexMutex.RUnlock()

	return u.lastCommittedEpochIndex
}

func (u *CommitmentState) SetLastCommittedEpoch(index epoch.Index) {
	u.lastCommittedEpochIndexMutex.Lock()
	defer u.lastCommittedEpochIndexMutex.Unlock()

	u.lastCommittedEpochIndex = index
}

func (u *CommitmentState) BatchedStateTransitionStarted() bool {
	u.batchEpochMutex.RLock()
	defer u.batchEpochMutex.RUnlock()

	return u.batchEpoch != 0
}
