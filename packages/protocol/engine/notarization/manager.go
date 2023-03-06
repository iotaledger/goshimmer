package notarization

import (
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/options"
)

const (
	defaultMinSlotCommittableAge = 6
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the component that manages the slot commitments.
type Manager struct {
	Events           *Events
	SlotMutations    *SlotMutations
	Attestations     *Attestations
	SlotTimeProvider *slot.TimeProvider

	storage         *storage.Storage
	ledgerState     *ledgerstate.LedgerState
	commitmentMutex sync.RWMutex

	acceptanceTime      time.Time
	acceptanceTimeMutex sync.RWMutex

	optsMinCommittableSlotAge slot.Index

	traits.Initializable
}

// NewManager creates a new notarization Manager.
func NewManager(storageInstance *storage.Storage, ledgerState *ledgerstate.LedgerState, weights *sybilprotection.Weights, slotTimeProvider *slot.TimeProvider, opts ...options.Option[Manager]) *Manager {
	return options.Apply(&Manager{
		Events:                    NewEvents(),
		SlotMutations:             NewSlotMutations(weights, storageInstance.Settings.LatestCommitment().Index()),
		Attestations:              NewAttestations(storageInstance.Permanent.Attestations, storageInstance.Prunable.Attestations, weights, slotTimeProvider),
		SlotTimeProvider:          slotTimeProvider,
		storage:                   storageInstance,
		ledgerState:               ledgerState,
		acceptanceTime:            slotTimeProvider.EndTime(storageInstance.Settings.LatestCommitment().Index()),
		optsMinCommittableSlotAge: defaultMinSlotCommittableAge,
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

	if index := m.SlotTimeProvider.IndexFromTime(acceptanceTime); index > m.storage.Settings.LatestCommitment().Index() {
		m.tryCommitSlotUntil(index)
	}
}

// IsFullyCommitted returns if the Manager finished committing all pending slots up to the current acceptance time.
func (m *Manager) IsFullyCommitted() bool {
	// If acceptance time is in slot 10, then the latest committable index is 3 (with optsMinCommittableSlotAge=6), because there are 6 full slots between slot 10 and slot 3.
	// All slots smaller than 4 are committable, so in order to check if slot 3 is committed it's necessary to do m.optsMinCommittableSlotAge-1,
	// otherwise we'd expect slot 4 to be committed in order to be fully committed, which is impossible.
	return m.storage.Settings.LatestCommitment().Index() >= m.SlotTimeProvider.IndexFromTime(m.AcceptanceTime())-m.optsMinCommittableSlotAge-1
}

func (m *Manager) NotarizeAcceptedBlock(block *models.Block) (err error) {
	if err = m.SlotMutations.AddAcceptedBlock(block); err != nil {
		return errors.Wrap(err, "failed to add accepted block to slot mutations")
	}

	if _, err = m.Attestations.Add(NewAttestation(block, m.SlotTimeProvider)); err != nil {
		return errors.Wrap(err, "failed to add block to attestations")
	}

	return
}

func (m *Manager) NotarizeOrphanedBlock(block *models.Block) (err error) {
	if err = m.SlotMutations.RemoveAcceptedBlock(block); err != nil {
		return errors.Wrap(err, "failed to remove accepted block from slot mutations")
	}

	if _, err = m.Attestations.Delete(NewAttestation(block, m.SlotTimeProvider)); err != nil {
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

func (m *Manager) Export(writer io.WriteSeeker, targetSlot slot.Index) (err error) {
	m.commitmentMutex.RLock()
	defer m.commitmentMutex.RUnlock()

	if err = m.Attestations.Export(writer, targetSlot); err != nil {
		return errors.Wrap(err, "failed to export attestations")
	}

	return
}

// MinCommittableSlotAge returns the minimum age of a slot to be committable.
func (m *Manager) MinCommittableSlotAge() slot.Index {
	return m.optsMinCommittableSlotAge
}

func (m *Manager) tryCommitSlotUntil(acceptedBlockIndex slot.Index) {
	for i := m.storage.Settings.LatestCommitment().Index() + 1; i <= acceptedBlockIndex; i++ {
		if !m.isCommittable(i, acceptedBlockIndex) {
			return
		}

		if !m.createCommitment(i) {
			return
		}
	}
}

func (m *Manager) isCommittable(index, acceptedBlockIndex slot.Index) bool {
	return index < acceptedBlockIndex-m.optsMinCommittableSlotAge
}

func (m *Manager) createCommitment(index slot.Index) (success bool) {
	latestCommitment := m.storage.Settings.LatestCommitment()
	if index != latestCommitment.Index()+1 {
		m.Events.Error.Trigger(errors.Errorf("cannot create commitment for slot %d, latest commitment is for slot %d", index, latestCommitment.Index()))

		return false
	}

	if err := m.ledgerState.ApplyStateDiff(index); err != nil {
		m.Events.Error.Trigger(errors.Wrapf(err, "failed to apply state diff for slot %d", index))
		return false
	}

	acceptedBlocks, acceptedTransactions, err := m.SlotMutations.Evict(index)
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
			m.SlotMutations.weights.Root(),
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

	m.Events.SlotCommitted.Trigger(&SlotCommittedDetails{
		Commitment:           newCommitment,
		AcceptedBlocks:       acceptedBlocks,
		AcceptedTransactions: acceptedTransactions,
		SpentOutputs: func(callback func(*ledger.OutputWithMetadata) error) error {
			return m.ledgerState.StateDiffs.StreamSpentOutputs(index, callback)
		},
		CreatedOutputs: func(callback func(*ledger.OutputWithMetadata) error) error {
			return m.ledgerState.StateDiffs.StreamCreatedOutputs(index, callback)
		},
		ActiveValidatorsCount: 0,
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

// WithMinCommittableSlotAge specifies how old an slot has to be for it to be committable.
func WithMinCommittableSlotAge(age slot.Index) options.Option[Manager] {
	return func(manager *Manager) {
		manager.optsMinCommittableSlotAge = age
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
