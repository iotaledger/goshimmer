package notarization

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

const (
	defaultMinEpochCommittableAge = 1 * time.Minute
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the component that manages the epoch commitments.
type Manager struct {
	Events         *Events
	EpochMutations *EpochMutations
	Attestations   *Attestations

	storage                  *storage.Storage
	ledgerState              *ledgerstate.LedgerState
	pendingConflictsCounters *shrinkingmap.ShrinkingMap[epoch.Index, uint64]
	commitmentMutex          sync.RWMutex

	acceptanceTime      time.Time
	acceptanceTimeMutex sync.RWMutex

	optsMinCommittableEpochAge time.Duration
}

// NewManager creates a new notarization Manager.
func NewManager(storageInstance *storage.Storage, ledgerState *ledgerstate.LedgerState, weights *sybilprotection.Weights, opts ...options.Option[Manager]) (newManager *Manager) {
	return options.Apply(&Manager{
		Events:                     NewEvents(),
		EpochMutations:             NewEpochMutations(weights, storageInstance.Settings.LatestCommitment().Index()),
		Attestations:               NewAttestations(storageInstance.Permanent.Attestations, storageInstance.Prunable.Attestations, weights),
		storage:                    storageInstance,
		ledgerState:                ledgerState,
		pendingConflictsCounters:   shrinkingmap.New[epoch.Index, uint64](),
		acceptanceTime:             storageInstance.Settings.LatestCommitment().Index().EndTime(),
		optsMinCommittableEpochAge: defaultMinEpochCommittableAge,
	}, opts)
}

// IncreaseConflictsCounter increases the conflicts counter for the given epoch index.
func (m *Manager) IncreaseConflictsCounter(index epoch.Index) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()

	if index <= m.storage.Settings.LatestCommitment().Index() {
		return
	}

	m.pendingConflictsCounters.Set(index, lo.Return1(m.pendingConflictsCounters.Get(index))+1)
}

// DecreaseConflictsCounter decreases the conflicts counter for the given epoch index.
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

// AcceptanceTime returns the acceptance time of the Manager.
func (m *Manager) AcceptanceTime() time.Time {
	m.acceptanceTimeMutex.RLock()
	defer m.acceptanceTimeMutex.RUnlock()

	return m.acceptanceTime
}

// SetAcceptanceTime sets the acceptance time of the Manager.
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

func (m *Manager) NotarizeAcceptedBlock(block *models.Block) (err error) {
	if err = m.EpochMutations.AddAcceptedBlock(block); err != nil {
		return errors.Errorf("failed to add accepted block to epoch mutations: %w", err)
	}

	if _, err = m.Attestations.Add(block); err != nil {
		return errors.Errorf("failed to add block to attestations: %w", err)
	}

	return
}

func (m *Manager) NotarizeOrphanedBlock(block *models.Block) (err error) {
	if err = m.EpochMutations.RemoveAcceptedBlock(block); err != nil {
		return errors.Errorf("failed to remove accepted block from epoch mutations: %w", err)
	}

	if _, err = m.Attestations.Delete(block); err != nil {
		return errors.Errorf("failed to delete block from attestations: %w", err)
	}

	return
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

	if err := m.ledgerState.ApplyStateDiff(index); err != nil {
		m.Events.Error.Trigger(errors.Errorf("failed to apply state diff for epoch %d: %w", index, err))
		return false
	}

	acceptedBlocks, acceptedTransactions, err := m.EpochMutations.Evict(index)
	if err != nil {
		m.Events.Error.Trigger(errors.Errorf("failed to commit mutations: %w", err))
		return false
	}

	attestations, attestationsWeight, err := m.Attestations.Commit(index)
	if err != nil {
		m.Events.Error.Trigger(errors.Errorf("failed to commit attestations: %w", err))
		return false
	}

	newCommitment := commitment.New(
		index,
		latestCommitment.ID(),
		commitment.NewRoots(
			acceptedBlocks.Root(),
			acceptedTransactions.Root(),
			attestations.Root(),
			m.ledgerState.UnspentOutputs.Root(),
			m.EpochMutations.weights.Root(),
		).ID(),
		m.storage.Settings.LatestCommitment().CumulativeWeight()+attestationsWeight,
	)

	if err = m.storage.Settings.SetLatestCommitment(newCommitment); err != nil {
		m.Events.Error.Trigger(errors.Errorf("failed to set latest commitment: %w", err))
	}

	m.Events.EpochCommitted.Trigger(&EpochCommittedDetails{
		Commitment:                newCommitment,
		AcceptedBlocksCount:       acceptedBlocks.Size(),
		AcceptedTransactionsCount: acceptedTransactions.Size(),
		ActiveValidatorsCount:     0,
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
