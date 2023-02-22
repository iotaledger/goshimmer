package notarization

import (
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/runtime/options"
)

const (
	defaultMinEpochCommittableAge = 6
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the component that manages the epoch commitments.
type Manager struct {
	Events         *Events
	EpochMutations *EpochMutations
	Attestations   *Attestations

	storage         *storage.Storage
	ledgerState     *ledgerstate.LedgerState
	commitmentMutex sync.RWMutex

	acceptanceTime      time.Time
	acceptanceTimeMutex sync.RWMutex

	optsMinCommittableEpochAge epoch.Index

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
		acceptanceTime:             storageInstance.Settings.LatestCommitment().Index().EndTime(),
		optsMinCommittableEpochAge: defaultMinEpochCommittableAge,
	}, opts, func(m *Manager) {
		m.Initializable = traits.NewInitializable(m.Attestations.TriggerInitialized)
	})
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
		m.tryCommitEpochUntil(index)
	}
}

// IsFullyCommitted returns if the Manager finished committing all pending epochs up to the current acceptance time.
func (m *Manager) IsFullyCommitted() bool {
	// If acceptance time is in epoch 10, then the latest committable index is 3 (with optsMinCommittableEpochAge=6), because there are 6 full epochs between epoch 10 and epoch 3.
	// All epochs smaller than 4 are committable, so in order to check if epoch 3 is committed it's necessary to do m.optsMinCommittableEpochAge-1,
	// otherwise we'd expect epoch 4 to be committed in order to be fully committed, which is impossible.
	return m.storage.Settings.LatestCommitment().Index() >= epoch.IndexFromTime(m.AcceptanceTime())-m.optsMinCommittableEpochAge-1
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
func (m *Manager) MinCommittableEpochAge() epoch.Index {
	return m.optsMinCommittableEpochAge
}

func (m *Manager) tryCommitEpochUntil(acceptedBlockIndex epoch.Index) {
	for i := m.storage.Settings.LatestCommitment().Index() + 1; i <= acceptedBlockIndex; i++ {
		if !m.isCommittable(i, acceptedBlockIndex) {
			return
		}

		if !m.createCommitment(i) {
			return
		}
	}
}

func (m *Manager) isCommittable(index, acceptedBlockIndex epoch.Index) (isCommittable bool) {
	return index < acceptedBlockIndex-m.optsMinCommittableEpochAge
}

func (m *Manager) createCommitment(index epoch.Index) (success bool) {
	latestCommitment := m.storage.Settings.LatestCommitment()
	if index != latestCommitment.Index()+1 {
		m.Events.Error.Trigger(errors.Errorf("cannot create commitment for epoch %d, latest commitment is for epoch %d", index, latestCommitment.Index()))

		return false
	}

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

	return true
}

func (m *Manager) PerformLocked(perform func(m *Manager)) {
	m.commitmentMutex.Lock()
	defer m.commitmentMutex.Unlock()
	perform(m)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithMinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func WithMinCommittableEpochAge(age epoch.Index) options.Option[Manager] {
	return func(manager *Manager) {
		manager.optsMinCommittableEpochAge = age
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
