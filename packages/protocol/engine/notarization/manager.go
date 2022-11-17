package notarization

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/storage"
)

const (
	defaultMinEpochCommittableAge = 1 * time.Minute
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type Manager struct {
	Events *Events
	*EpochMutations

	storage                  *storage.Storage
	pendingConflictsCounters *shrinkingmap.ShrinkingMap[epoch.Index, uint64]
	commitmentMutex          sync.RWMutex

	acceptanceTime      time.Time
	acceptanceTimeMutex sync.RWMutex

	optsMinCommittableEpochAge time.Duration
}

func NewManager(storage *storage.Storage, sybilProtection *sybilprotection.SybilProtection, opts ...options.Option[Manager]) (new *Manager) {
	return options.Apply(&Manager{
		Events:         NewEvents(),
		EpochMutations: NewEpochMutations(sybilProtection.Weight, storage.Settings.LatestCommitment().Index()),

		storage:                    storage,
		pendingConflictsCounters:   shrinkingmap.New[epoch.Index, uint64](),
		acceptanceTime:             storage.Settings.LatestCommitment().Index().EndTime(),
		optsMinCommittableEpochAge: defaultMinEpochCommittableAge,
	}, opts)
}

func (m *Manager) IncreaseConflictsCounter(index epoch.Index) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()

	if index <= m.storage.Settings.LatestCommitment().Index() {
		return
	}

	m.pendingConflictsCounters.Set(index, lo.Return1(m.pendingConflictsCounters.Get(index))+1)
}

func (m *Manager) DecreaseConflictsCounter(index epoch.Index) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()

	if index <= m.storage.Settings.LatestCommitment().Index() {
		return
	}

	if newCounter := lo.Return1(m.pendingConflictsCounters.Get(index)) - 1; newCounter != 0 {
		m.pendingConflictsCounters.Set(index, newCounter)
	} else {
		m.pendingConflictsCounters.Delete(index)

		m.tryCommitEpoch(index, m.AcceptanceTime())
	}
}

func (m *Manager) AcceptanceTime() time.Time {
	m.acceptanceTimeMutex.Lock()
	defer m.acceptanceTimeMutex.Unlock()

	return m.acceptanceTime
}

func (m *Manager) SetAcceptanceTime(acceptanceTime time.Time) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()

	m.acceptanceTimeMutex.Lock()
	m.acceptanceTime = acceptanceTime
	m.acceptanceTimeMutex.Unlock()

	if index := epoch.IndexFromTime(acceptanceTime); index > m.storage.Settings.LatestCommitment().Index() {
		m.tryCommitEpoch(index, acceptanceTime)
	}
}

// IsFullyCommitted returns if the Manager finished committing all pending epochs up to the current acceptance time.
func (m *Manager) IsFullyCommitted() bool {
	return m.AcceptanceTime().Sub((m.storage.Settings.LatestCommitment().Index() + 1).EndTime()) < m.optsMinCommittableEpochAge
}

func (m *Manager) tryCommitEpoch(index epoch.Index, acceptanceTime time.Time) {
	for i := m.storage.Settings.LatestCommitment().Index() + 1; i <= index; i++ {
		if !m.isCommittable(i, acceptanceTime) || !m.createCommitment(i) {
			return
		}
	}
}

func (m *Manager) isCommittable(ei epoch.Index, acceptanceTime time.Time) (isCommittable bool) {
	return acceptanceTime.Sub(ei.EndTime()) >= m.optsMinCommittableEpochAge && m.hasNoPendingConflicts(ei)
}

func (m *Manager) hasNoPendingConflicts(ei epoch.Index) (hasNoPendingConflicts bool) {
	for index := m.storage.Settings.LatestCommitment().Index(); index <= ei; index++ {
		if count, _ := m.pendingConflictsCounters.Get(index); count != 0 {
			return false
		}
	}

	return true
}

func (m *Manager) createCommitment(index epoch.Index) (success bool) {
	latestCommitment := m.storage.Settings.LatestCommitment()
	if index != latestCommitment.Index()+1 {
		m.Events.Error.Trigger(errors.Errorf("cannot create commitment for epoch %d, latest commitment is for epoch %d", index, latestCommitment.Index()))

		return false
	}

	m.pendingConflictsCounters.Delete(index)

	stateRoot, manaRoot := m.storage.ApplyStateDiff(index, m.storage.LedgerStateDiffs.StateDiff(index))

	acceptedBlocks, acceptedTransactions, activeValidators, cumulativeWeight, err := m.EpochMutations.Commit(index)
	if err != nil {
		m.Events.Error.Trigger(errors.Errorf("failed to commit mutations: %w", err))

		return false
	}

	newCommitment := commitment.New(index, latestCommitment.ID(), commitment.NewRoots(acceptedBlocks.Root(), acceptedTransactions.Root(), activeValidators.Root(), stateRoot, manaRoot).ID(), cumulativeWeight)

	if err = m.storage.Settings.SetLatestCommitment(newCommitment); err != nil {
		m.Events.Error.Trigger(errors.Errorf("failed to set latest commitment: %w", err))
	}

	fmt.Println("epoch commited", newCommitment.Index())
	m.Events.EpochCommitted.Trigger(newCommitment)
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// MinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func MinCommittableEpochAge(d time.Duration) options.Option[Manager] {
	return func(manager *Manager) {
		manager.optsMinCommittableEpochAge = d
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
