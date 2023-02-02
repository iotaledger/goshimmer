package notarization

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
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
	pendingConflictsCounters *memstorage.EpochStorage[models.BlockID, *models.Block]
	commitmentMutex          sync.RWMutex

	acceptanceTime      time.Time
	acceptanceTimeMutex sync.RWMutex

	optsMinCommittableEpochAge time.Duration

	traits.Initializable
}

// NewManager creates a new notarization Manager.
func NewManager(storageInstance *storage.Storage, ledgerState *ledgerstate.LedgerState, weights *sybilprotection.Weights, opts ...options.Option[Manager]) (newManager *Manager) {
	return options.Apply(&Manager{
		Events:                     NewEvents(),
		EpochMutations:             NewEpochMutations(weights, storageInstance.Settings.LatestCommitment().Index()),
		Attestations:               NewAttestations(storageInstance.Permanent.Attestations, storageInstance.Prunable.Attestations, weights),
		storage:                    storageInstance,
		ledgerState:                ledgerState,
		pendingConflictsCounters:   memstorage.NewEpochStorage[models.BlockID, *models.Block](),
		acceptanceTime:             storageInstance.Settings.LatestCommitment().Index().EndTime(),
		optsMinCommittableEpochAge: defaultMinEpochCommittableAge,
	}, opts, func(m *Manager) {
		m.Initializable = traits.NewInitializable(m.Attestations.TriggerInitialized)
	})
}

// AddConflictingAttachment adds the conflicting attachment to a set of pending conflicts of an epoch.
func (m *Manager) AddConflictingAttachment(block *models.Block) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()

	if block.ID().Index() <= m.storage.Settings.LatestCommitment().Index() {
		return
	}

	pendingConflicts := m.pendingConflictsCounters.Get(block.ID().Index(), true)
	pendingConflicts.Set(block.ID(), block)
}

// DeleteConflictingAttachment deletes the conflicting attachment from a set of pending conflicts of an epoch.
func (m *Manager) DeleteConflictingAttachment(blockID models.BlockID) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()

	if blockID.Index() <= m.storage.Settings.LatestCommitment().Index() {
		return
	}

	pendingConflicts := m.pendingConflictsCounters.Get(blockID.Index())
	if pendingConflicts != nil {
		pendingConflicts.Delete(blockID)
	}

	if pendingConflicts == nil || pendingConflicts.IsEmpty() {
		m.tryCommitEpoch(blockID.Index(), m.AcceptanceTime())
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
		return errors.Wrap(err, "failed to add accepted block to epoch mutations")
	}

	if _, err = m.Attestations.Add(NewAttestation(block)); err != nil {
		return errors.Wrap(err, "failed to add block to attestations")
	}

	return
}

func (m *Manager) NotarizeOrphanedBlock(block *models.Block) (err error) {
	if err = m.EpochMutations.RemoveAcceptedBlock(block); err != nil {
		return errors.Wrap(err, "failed to remove accepted block from epoch mutations")
	}

	if _, err = m.Attestations.Delete(NewAttestation(block)); err != nil {
		return errors.Wrap(err, "failed to delete block from attestations")
	}

	return
}

func (m *Manager) Import(reader io.ReadSeeker) (err error) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()

	if err = m.Attestations.Import(reader); err != nil {
		return errors.Wrap(err, "failed to import attestations")
	}

	m.TriggerInitialized()

	return
}

func (m *Manager) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	m.commitmentMutex.RLock()
	defer m.commitmentMutex.RUnlock()

	if err = m.Attestations.Export(writer, targetEpoch); err != nil {
		return errors.Wrap(err, "failed to export attestations")
	}

	return
}

// MinCommittableEpochAge returns the minimum age of an epoch to be committable.
func (m *Manager) MinCommittableEpochAge() time.Duration {
	return m.optsMinCommittableEpochAge
}

func (m *Manager) tryCommitEpoch(index epoch.Index, acceptanceTime time.Time) {
	for i := m.storage.Settings.LatestCommitment().Index() + 1; i <= index; i++ {
		if !m.isCommittable(i, acceptanceTime) {
			return
		}
		fmt.Println(">> committing", i)

		// drop still-pending conflicts of an epoch old enough to be committed.
		m.dropPendingConflicts(i)

		if !m.createCommitment(i) {
			fmt.Println(">> epoch not committable", i)
			return
		}
	}
}

func (m *Manager) isCommittable(ei epoch.Index, acceptanceTime time.Time) (isCommittable bool) {
	return acceptanceTime.Sub(ei.EndTime()) >= m.optsMinCommittableEpochAge
}

func (m *Manager) dropPendingConflicts(index epoch.Index) {
	if storage := m.pendingConflictsCounters.Get(index); storage != nil {
		storage.ForEach(func(attachmentID models.BlockID, attachment *models.Block) bool {
			tx, _ := attachment.Payload().(utxo.Transaction)
			fmt.Printf(">> dropping attachment %s, TXID %s\n", attachmentID, tx.ID())
			m.Events.ConflictDropped.Trigger(attachmentID)
			return true
		})
		m.pendingConflictsCounters.Evict(index)
	}
}

func (m *Manager) createCommitment(index epoch.Index) (success bool) {
	latestCommitment := m.storage.Settings.LatestCommitment()
	if index != latestCommitment.Index()+1 {
		m.Events.Error.Trigger(errors.Errorf("cannot create commitment for epoch %d, latest commitment is for epoch %d", index, latestCommitment.Index()))

		return false
	}

	// TODO: remove
	// This should have been already evicted by dropPendingConflicts.
	// m.pendingConflictsCounters.Evict(index)

	if err := m.ledgerState.ApplyStateDiff(index); err != nil {
		m.Events.Error.Trigger(errors.Wrapf(err, "failed to apply state diff for epoch %d", index))
		return false
	}

	acceptedBlocks, acceptedTransactions, err := m.EpochMutations.Evict(index)
	if err != nil {
		m.Events.Error.Trigger(errors.Wrap(err, "failed to commit mutations"))
		return false
	}

	attestations, attestationsWeight, err := m.Attestations.Commit(index)
	if err != nil {
		m.Events.Error.Trigger(errors.Wrap(err, "failed to commit attestations"))
		return false
	}

	fmt.Println(">> ROOTS",
		acceptedBlocks.Root(),
		acceptedTransactions.Root(),
		attestations.Root(),
		m.ledgerState.UnspentOutputs.Root(),
		m.EpochMutations.weights.Root(),
	)

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
		m.Events.Error.Trigger(errors.Wrap(err, "failed to set latest commitment"))
		return false
	}

	if err = m.storage.Commitments.Store(newCommitment); err != nil {
		m.Events.Error.Trigger(errors.Wrap(err, "failed to store latest commitment"))
		return false
	}

	m.Events.EpochCommitted.Trigger(&EpochCommittedDetails{
		Commitment:                newCommitment,
		AcceptedBlocksCount:       acceptedBlocks.Size(),
		AcceptedTransactionsCount: acceptedTransactions.Size(),
		ActiveValidatorsCount:     0,
	})

	fmt.Println(">> epoch committed", index)

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithMinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func WithMinCommittableEpochAge(d time.Duration) options.Option[Manager] {
	return func(manager *Manager) {
		manager.optsMinCommittableEpochAge = d
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
