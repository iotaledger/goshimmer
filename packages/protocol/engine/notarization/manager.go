package notarization

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

const (
	defaultMinEpochCommittableAge = 1 * time.Minute
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

type Manager struct {
	Events *Events
	*EpochMutations

	storage                    *chainstorage.ChainStorage
	pendingConflictsCounters   *shrinkingmap.ShrinkingMap[epoch.Index, uint64]
	acceptanceTime             time.Time
	optsMinCommittableEpochAge time.Duration

	sync.RWMutex
}

func NewManager(storage *chainstorage.ChainStorage, opts ...options.Option[Manager]) (new *Manager) {
	return options.Apply(&Manager{
		Events:         NewEvents(),
		EpochMutations: NewEpochMutations(storage.LatestCommitment().Index()),

		storage:                    storage,
		pendingConflictsCounters:   shrinkingmap.New[epoch.Index, uint64](),
		optsMinCommittableEpochAge: defaultMinEpochCommittableAge,
	}, opts)
}

func (m *Manager) IncreaseConflictsCounter(index epoch.Index) {
	m.Lock()
	defer m.Unlock()

	if index <= m.storage.LatestCommitment().Index() {
		return
	}

	m.pendingConflictsCounters.Set(index, lo.Return1(m.pendingConflictsCounters.Get(index))+1)
}

func (m *Manager) DecreaseConflictsCounter(index epoch.Index) {
	m.Lock()
	defer m.Unlock()

	if index <= m.storage.LatestCommitment().Index() {
		return
	}

	if newCounter := lo.Return1(m.pendingConflictsCounters.Get(index)) - 1; newCounter != 0 {
		m.pendingConflictsCounters.Set(index, newCounter)
	} else {
		m.pendingConflictsCounters.Delete(index)

		m.tryCommitEpoch(index)
	}
}

func (m *Manager) SetAcceptanceTime(acceptanceTime time.Time) {
	m.Lock()
	defer m.Unlock()

	m.acceptanceTime = acceptanceTime

	if index := epoch.IndexFromTime(acceptanceTime); index > m.storage.LatestCommitment().Index() {
		m.tryCommitEpoch(index)
	}
}

func (m *Manager) tryCommitEpoch(index epoch.Index) {
	for i := m.storage.LatestCommitment().Index() + 1; i <= index; i++ {
		if !m.isCommittable(i) || !m.createCommitment(i) {
			return
		}
	}
}

func (m *Manager) isCommittable(ei epoch.Index) (isCommittable bool) {
	return m.acceptanceTime.Sub(ei.EndTime()) >= m.optsMinCommittableEpochAge && m.hasNoPendingConflicts(ei)
}

func (m *Manager) hasNoPendingConflicts(ei epoch.Index) (hasNoPendingConflicts bool) {
	for index := m.storage.LatestCommitment().Index(); index <= ei; index++ {
		if count, _ := m.pendingConflictsCounters.Get(index); count != 0 {
			return false
		}
	}

	return true
}

func (m *Manager) createCommitment(index epoch.Index) (success bool) {
	latestCommitment := m.storage.LatestCommitment()
	if index != latestCommitment.Index()+1 {
		m.Events.Error.Trigger(errors.Errorf("cannot create commitment for epoch %d, latest commitment is for epoch %d", index, latestCommitment.Index()))

		return false
	}

	acceptedBlocks, acceptedTransactions, activeValidators, err := m.EpochMutations.Commit(index)
	if err != nil {
		m.Events.Error.Trigger(errors.Errorf("failed to commit mutations: %w", err))

		return false
	}
	stateRoot, manaRoot := m.storage.State.Apply(m.storage.DiffStorage.StateDiff(index))

	// TODO: obtain and commit to cumulative weight
	newCommitment := commitment.New(index, latestCommitment.ID(), commitment.NewRoots(acceptedBlocks.Root(), acceptedTransactions.Root(), activeValidators.Root(), stateRoot, manaRoot).ID(), 0)

	m.storage.SetLatestCommitment(newCommitment)

	m.pendingConflictsCounters.Delete(index)

	// TODO: FIX UPDATES
	m.Events.ConsensusWeightsUpdated.Trigger(&ConsensusWeightsUpdatedEvent{
		EI:                      index,
		AmountAndDiffByIdentity: nil,
	})

	m.Events.EpochCommitted.Trigger(&EpochCommittedEvent{
		EI:         index,
		Commitment: newCommitment,
	})

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
