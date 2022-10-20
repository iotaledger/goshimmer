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
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

const (
	defaultMinEpochCommittableAge = 1 * time.Minute
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the notarization manager.
type Manager struct {
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
	m.Lock()
	defer m.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.Events.Error.Trigger(errors.Errorf("conflict confirmed in already committed epoch %d", ei))
		return
	}

	m.decreasePendingConflictCounter(ei)
}

// OnConflictCreated is the handler for conflict created event.
func (m *Manager) OnConflictCreated(conflictID utxo.TransactionID) {
	m.Lock()
	defer m.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.Events.Error.Trigger(errors.Errorf("conflict created in already committed epoch %d", ei))
		return
	}
	m.increasePendingConflictCounter(ei)
}

// OnConflictRejected is the handler for conflict created event.
func (m *Manager) OnConflictRejected(conflictID utxo.TransactionID) {
	m.Lock()
	defer m.Unlock()

	ei := m.getConflictEI(conflictID)

	if m.isEpochAlreadyCommitted(ei) {
		m.Events.Error.Trigger(errors.Errorf("conflict rejected in already committed epoch %d", ei))
		return
	}

	m.decreasePendingConflictCounter(ei)
}

// OnAcceptanceTimeUpdated is the handler for time updated event and triggers the events.
func (m *Manager) OnAcceptanceTimeUpdated(newTime time.Time) {
	m.Lock()
	defer m.Unlock()

	if newTimeEpoch := epoch.IndexFromTime(newTime); newTimeEpoch > m.chainStorage.LatestCommittedEpoch() {
		m.moveLatestCommittableEpoch(newTimeEpoch)
	}
}

// PendingConflictsCountAll returns the current value of pendingConflictsCount per epoch.
func (m *Manager) PendingConflictsCountAll() (pendingConflicts map[epoch.Index]uint64) {
	m.RLock()
	defer m.RUnlock()

	pendingConflicts = make(map[epoch.Index]uint64, m.pendingConflictsCounters.Size())
	m.pendingConflictsCounters.ForEach(func(k epoch.Index, v uint64) bool {
		pendingConflicts[k] = v
		return true
	})
	return pendingConflicts
}

func (m *Manager) decreasePendingConflictCounter(ei epoch.Index) {
	count, _ := m.pendingConflictsCounters.Get(ei)
	count--
	m.pendingConflictsCounters.Set(ei, count)
	if count == 0 {
		m.moveLatestCommittableEpoch(ei)
	}
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
	latestCommittedEpoch := m.chainStorage.LatestCommittedEpoch()

	for index := latestCommittedEpoch; index <= ei; index++ {
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

	return currentATT.Sub(t) >= m.optsMinCommittableEpochAge
}

func (m *Manager) getConflictEI(conflictID utxo.TransactionID) epoch.Index {
	earliestAttachment := m.tangle.GetEarliestAttachment(conflictID)
	return epoch.IndexFromTime(earliestAttachment.IssuingTime())
}

func (m *Manager) isEpochAlreadyCommitted(ei epoch.Index) bool {
	return ei <= m.chainStorage.LatestCommittedEpoch()
}

func (m *Manager) moveLatestCommittableEpoch(currentEpoch epoch.Index) {
	for ei := m.chainStorage.LatestCommittedEpoch() + 1; ei <= currentEpoch; ei++ {
		if !m.isCommittable(ei) {
			break
		}

		// TODO: compact diffstorage
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
			return
		}

		m.chainStorage.SetLatestCommittedEpoch(ei)

		m.Events.EpochCommitted.Trigger(&EpochCommittedEvent{
			EI:         ei,
			Commitment: newCommitment,
		})

		// We do not need to track pending conflicts for a committed epoch anymore.
		m.pendingConflictsCounters.Delete(ei)
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
