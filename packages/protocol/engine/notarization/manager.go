package notarization

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/mana"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

const (
	defaultMinEpochCommittableAge = 1 * time.Minute
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the notarization manager.
type Manager struct {
	Events *Events

	clock                    *clock.Clock
	tangle                   *tangle.Tangle
	ledger                   *ledger.Ledger
	consensus                *consensus.Consensus
	chainStorage             *chainstorage.ChainStorage
	genesisCommitment        *commitment.Commitment
	bootstrapMutex           sync.RWMutex
	pendingConflictsCounters *shrinkingmap.ShrinkingMap[epoch.Index, uint64]

	optsMinCommittableEpochAge time.Duration
	optsBootstrapWindow        time.Duration
	optsManaEpochDelay         uint

	*commitmentFactory
}

// NewManager creates and returns a new notarization manager.
func NewManager(c *clock.Clock, t *tangle.Tangle, l *ledger.Ledger, consensusInstance *consensus.Consensus, chainStorage *chainstorage.ChainStorage, genesisCommitment *commitment.Commitment, opts ...options.Option[Manager]) (new *Manager) {
	return options.Apply(&Manager{
		clock:                    c,
		tangle:                   t,
		ledger:                   l,
		consensus:                consensusInstance,
		chainStorage:             chainStorage,
		genesisCommitment:        genesisCommitment,
		pendingConflictsCounters: shrinkingmap.New[epoch.Index, uint64](),
		Events:                   NewEvents(),

		optsMinCommittableEpochAge: defaultMinEpochCommittableAge,
	}, opts,
		(*Manager).initCommitmentFactory,
	)
}

func (m *Manager) initCommitmentFactory() {
	m.commitmentFactory = newCommitmentFactory(m.genesisCommitment, m.chainStorage)
}

// OnConflictAccepted is the handler for conflict confirmed event.
func (m *Manager) OnConflictAccepted(conflictID utxo.TransactionID) {
	epochCommittableEvents, manaVectorUpdateEvents := m.onConflictAccepted(conflictID)
	m.triggerEpochEvents(epochCommittableEvents, manaVectorUpdateEvents)
}

// OnConflictConfirmed is the handler for conflict confirmed event.
func (m *Manager) onConflictAccepted(conflictID utxo.TransactionID) ([]*EpochCommittedEvent, []*mana.ManaVectorUpdateEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.Events.Error.Trigger(errors.Errorf("conflict confirmed in already committed epoch %d", ei))
		return nil, nil
	}
	return m.decreasePendingConflictCounter(ei)
}

// OnConflictCreated is the handler for conflict created event.
func (m *Manager) OnConflictCreated(conflictID utxo.TransactionID) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.Events.Error.Trigger(errors.Errorf("conflict created in already committed epoch %d", ei))
		return
	}
	m.increasePendingConflictCounter(ei)
}

// OnConflictRejected is the handler for conflict created event.
func (m *Manager) OnConflictRejected(conflictID utxo.TransactionID) {
	epochCommittableEvents, manaVectorUpdateEvents := m.onConflictRejected(conflictID)
	m.triggerEpochEvents(epochCommittableEvents, manaVectorUpdateEvents)
}

// OnConflictRejected is the handler for conflict created event.
func (m *Manager) onConflictRejected(conflictID utxo.TransactionID) ([]*EpochCommittedEvent, []*mana.ManaVectorUpdateEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.Events.Error.Trigger(errors.Errorf("conflict rejected in already committed epoch %d", ei))
		return nil, nil
	}
	return m.decreasePendingConflictCounter(ei)
}

// OnAcceptanceTimeUpdated is the handler for time updated event and triggers the events.
func (m *Manager) OnAcceptanceTimeUpdated(newTime time.Time) {
	epochCommittedEvents, manaVectorUpdateEvents := m.onAcceptanceTimeUpdated(newTime)
	m.triggerEpochEvents(epochCommittedEvents, manaVectorUpdateEvents)
}

// OnAcceptanceTimeUpdated is the handler for time updated event and returns events to be triggered.
func (m *Manager) onAcceptanceTimeUpdated(newTime time.Time) ([]*EpochCommittedEvent, []*mana.ManaVectorUpdateEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(newTime)
	currentEpochIndex, err := m.commitmentFactory.storage.acceptanceEpochIndex()
	if err != nil {
		m.Events.Error.Trigger(errors.Wrap(err, "could not get current epoch index"))
		return nil, nil
	}
	// moved to the next epoch
	if ei > currentEpochIndex {
		err = m.commitmentFactory.storage.setAcceptanceEpochIndex(ei)
		if err != nil {
			m.Events.Error.Trigger(errors.Wrap(err, "could not set current epoch index"))
			return nil, nil
		}
		return m.moveLatestCommittableEpoch(ei)
	}
	return nil, nil
}

// PendingConflictsCountAll returns the current value of pendingConflictsCount per epoch.
func (m *Manager) PendingConflictsCountAll() (pendingConflicts map[epoch.Index]uint64) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	pendingConflicts = make(map[epoch.Index]uint64, m.pendingConflictsCounters.Size())
	m.pendingConflictsCounters.ForEach(func(k epoch.Index, v uint64) bool {
		pendingConflicts[k] = v
		return true
	})
	return pendingConflicts
}

// Shutdown shuts down the manager's permanent storagee.
func (m *Manager) Shutdown() {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	m.commitmentFactory.storage.shutdown()
}

func (m *Manager) decreasePendingConflictCounter(ei epoch.Index) ([]*EpochCommittedEvent, []*mana.ManaVectorUpdateEvent) {
	count, _ := m.pendingConflictsCounters.Get(ei)
	count--
	m.pendingConflictsCounters.Set(ei, count)
	if count == 0 {
		return m.moveLatestCommittableEpoch(ei)
	}
	return nil, nil
}

func (m *Manager) increasePendingConflictCounter(ei epoch.Index) {
	count, _ := m.pendingConflictsCounters.Get(ei)
	count++
	m.pendingConflictsCounters.Set(ei, count)
}

// isCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) isCommittable(ei epoch.Index) bool {
	return m.isOldEnough(ei) && m.allPastConflictsAreResolved(ei)
}

func (m *Manager) allPastConflictsAreResolved(ei epoch.Index) (conflictsResolved bool) {
	lastEI, err := m.commitmentFactory.storage.latestCommittableEpochIndex()
	if err != nil {
		return false
	}
	// epoch is not committable if there are any not resolved conflicts in this and past epochs
	for index := lastEI; index <= ei; index++ {
		if count, _ := m.pendingConflictsCounters.Get(index); count != 0 {
			return false
		}
	}
	return true
}

func (m *Manager) isOldEnough(ei epoch.Index, issuingTime ...time.Time) (oldEnough bool) {
	t := ei.EndTime()
	currentATT := m.clock.AcceptedTime()
	if len(issuingTime) > 0 && issuingTime[0].After(currentATT) {
		currentATT = issuingTime[0]
	}

	diff := currentATT.Sub(t)
	if diff < m.optsMinCommittableEpochAge {
		return false
	}
	return true
}

func (m *Manager) getConflictEI(conflictID utxo.TransactionID) epoch.Index {
	earliestAttachment := m.tangle.GetEarliestAttachment(conflictID)
	return epoch.IndexFromTime(earliestAttachment.IssuingTime())
}

func (m *Manager) isEpochAlreadyCommitted(ei epoch.Index) bool {
	return ei <= m.chainStorage.LatestCommittedEpoch()
}

func (m *Manager) resolveOutputs(tx utxo.Transaction) (spentOutputsWithMetadata, createdOutputsWithMetadata []*chainstorage.OutputWithMetadata) {
	spentOutputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	createdOutputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	var spentOutputIDs utxo.OutputIDs
	var createdOutputs []utxo.Output

	spentOutputIDs = m.ledger.Utils.ResolveInputs(tx.Inputs())
	createdOutputs = tx.(*devnetvm.Transaction).Essence().Outputs().UTXOOutputs()

	for it := spentOutputIDs.Iterator(); it.HasNext(); {
		spentOutputID := it.Next()
		m.ledger.Storage.CachedOutput(spentOutputID).Consume(func(spentOutput utxo.Output) {
			m.ledger.Storage.CachedOutputMetadata(spentOutputID).Consume(func(spentOutputMetadata *ledger.OutputMetadata) {
				spentOutputsWithMetadata = append(spentOutputsWithMetadata, ledger.NewOutputWithMetadata(spentOutputID, spentOutput, spentOutputMetadata.CreationTime(), spentOutputMetadata.ConsensusManaPledgeID(), spentOutputMetadata.AccessManaPledgeID()))
			})
		})
	}

	for _, createdOutput := range createdOutputs {
		createdOutputID := createdOutput.ID()
		m.ledger.Storage.CachedOutputMetadata(createdOutputID).Consume(func(createdOutputMetadata *ledger.OutputMetadata) {
			createdOutputsWithMetadata = append(createdOutputsWithMetadata, ledger.NewOutputWithMetadata(createdOutputID, createdOutput, createdOutputMetadata.CreationTime(), createdOutputMetadata.ConsensusManaPledgeID(), createdOutputMetadata.AccessManaPledgeID()))
		})
	}

	return
}

func (m *Manager) manaVectorUpdate(ei epoch.Index) (event *mana.ManaVectorUpdateEvent) {
	manaEpoch := ei - epoch.Index(m.optsManaEpochDelay)
	spent := []*ledger.OutputWithMetadata{}
	created := []*ledger.OutputWithMetadata{}

	if manaEpoch > 0 {
		spent, created = m.commitmentFactory.loadDiffUTXOs(manaEpoch)
	}

	return &mana.ManaVectorUpdateEvent{
		EI:      ei,
		Spent:   spent,
		Created: created,
	}
}

func (m *Manager) moveLatestCommittableEpoch(currentEpoch epoch.Index) ([]*EpochCommittedEvent, []*mana.ManaVectorUpdateEvent) {
	epochCommittedEvents := make([]*EpochCommittedEvent, 0)
	manaVectorUpdateEvents := make([]*mana.ManaVectorUpdateEvent, 0)
	for ei := m.chainStorage.LatestCommittedEpoch() + 1; ei <= currentEpoch; ei++ {
		if !m.isCommittable(ei) {
			break
		}

		spentOutputsWithMetadata := make([]*chainstorage.OutputWithMetadata, 0)
		m.chainStorage.DiffStorage.StreamSpent(ei, func(outputWithMetadata *chainstorage.OutputWithMetadata) {
			spentOutputsWithMetadata = append(spentOutputsWithMetadata, outputWithMetadata)
		})
		createdOutputsWithMetadata := make([]*chainstorage.OutputWithMetadata, 0)
		m.chainStorage.DiffStorage.StreamCreated(ei, func(outputWithMetadata *chainstorage.OutputWithMetadata) {
			spentOutputsWithMetadata = append(createdOutputsWithMetadata, outputWithMetadata)
		})

		newCommitment, newCommitmentErr := m.commitmentFactory.createCommitment(ei, spentOutputsWithMetadata, createdOutputsWithMetadata)
		if newCommitmentErr != nil {
			m.Events.Error.Trigger(errors.Errorf("could not update commitments for epoch %d: %v", ei, newCommitmentErr))
			return nil, nil
		}

		m.chainStorage.SetLatestCommittedEpoch(ei)

		epochCommittedEvents = append(epochCommittedEvents, &EpochCommittedEvent{
			EI:         ei,
			Commitment: newCommitment,
		})
		if manaVectorUpdateEvent := m.manaVectorUpdate(ei); manaVectorUpdateEvent != nil {
			manaVectorUpdateEvents = append(manaVectorUpdateEvents, manaVectorUpdateEvent)
		}

		// We do not need to track pending conflicts for a committed epoch anymore.
		m.pendingConflictsCounters.Delete(ei)
	}
	return epochCommittedEvents, manaVectorUpdateEvents
}

func (m *Manager) triggerEpochEvents(epochCommittableEvents []*EpochCommittedEvent, manaVectorUpdateEvents []*mana.ManaVectorUpdateEvent) {
	for _, epochCommittableEvent := range epochCommittableEvents {
		m.Events.EpochCommitted.Trigger(epochCommittableEvent)
	}
	for _, manaVectorUpdateEvent := range manaVectorUpdateEvents {
		m.Events.ManaVectorUpdate.Trigger(manaVectorUpdateEvent)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// ManagerOption represents the return type of the optional config parameters of the notarization manager.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container of all the config parameters of the notarization manager.
type ManagerOptions struct{}

// ManaEpochDelay specifies how many epochs the consensus mana booking is delayed with respect to the latest committable
// epoch.
func ManaEpochDelay(manaEpochDelay uint) options.Option[Manager] {
	return func(manager *Manager) {
		manager.optsManaEpochDelay = manaEpochDelay
	}
}

// MinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func MinCommittableEpochAge(d time.Duration) options.Option[Manager] {
	return func(manager *Manager) {
		manager.optsMinCommittableEpochAge = d
	}
}

// BootstrapWindow specifies when the notarization manager is considered to be bootstrapped.
func BootstrapWindow(d time.Duration) options.Option[Manager] {
	return func(manager *Manager) {
		manager.optsBootstrapWindow = d
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
