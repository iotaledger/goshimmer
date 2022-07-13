package notarization

import (
	"github.com/iotaledger/hive.go/identity"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	defaultMinEpochCommittableAge = 1 * time.Minute
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the notarization manager.
type Manager struct {
	tangle                      *tangle.Tangle
	epochCommitmentFactory      *EpochCommitmentFactory
	epochCommitmentFactoryMutex sync.RWMutex
	options                     *ManagerOptions
	pendingConflictsCounters    map[epoch.Index]uint64
	log                         *logger.Logger
	Events                      *Events
}

// NewManager creates and returns a new notarization manager.
func NewManager(epochCommitmentFactory *EpochCommitmentFactory, t *tangle.Tangle, opts ...ManagerOption) (new *Manager) {
	options := &ManagerOptions{
		MinCommittableEpochAge: defaultMinEpochCommittableAge,
		Log:                    nil,
	}

	for _, option := range opts {
		option(options)
	}

	new = &Manager{
		tangle:                   t,
		epochCommitmentFactory:   epochCommitmentFactory,
		pendingConflictsCounters: make(map[epoch.Index]uint64),
		log:                      options.Log,
		options:                  options,
		Events: &Events{
			TangleTreeInserted:        event.New[*TangleTreeUpdatedEvent](),
			TangleTreeRemoved:         event.New[*TangleTreeUpdatedEvent](),
			StateMutationTreeInserted: event.New[*StateMutationTreeUpdatedEvent](),
			StateMutationTreeRemoved:  event.New[*StateMutationTreeUpdatedEvent](),
			UTXOTreeInserted:          event.New[*UTXOUpdatedEvent](),
			UTXOTreeRemoved:           event.New[*UTXOUpdatedEvent](),
			EpochCommittable:          event.New[*EpochCommittableEvent](),
			ManaVectorUpdate:          event.New[*ManaVectorUpdateEvent](),
			ActivityTreeInserted:      event.New[*ActivityTreeUpdatedEvent](),
			ActivityTreeRemoved:       event.New[*ActivityTreeUpdatedEvent](),
		},
	}

	new.tangle.ConfirmationOracle.Events().MessageAccepted.Attach(onlyIfBootstrapped(t.TimeManager, func(event *tangle.MessageAcceptedEvent) {
		new.OnMessageConfirmed(event.Message)
	}))

	new.tangle.ConfirmationOracle.Events().MessageOrphaned.Attach(onlyIfBootstrapped(t.TimeManager, func(event *tangle.MessageAcceptedEvent) {
		new.OnMessageOrphaned(event.Message)
	}))

	new.tangle.Ledger.Events.TransactionAccepted.Attach(onlyIfBootstrapped(t.TimeManager, func(event *ledger.TransactionAcceptedEvent) {
		new.OnTransactionAccepted(event)
	}))

	new.tangle.Ledger.Events.TransactionInclusionUpdated.Attach(onlyIfBootstrapped(t.TimeManager, func(event *ledger.TransactionInclusionUpdatedEvent) {
		new.OnTransactionInclusionUpdated(event)
	}))

	new.tangle.Ledger.ConflictDAG.Events.BranchAccepted.Attach(onlyIfBootstrapped(t.TimeManager, func(event *conflictdag.BranchAcceptedEvent[utxo.TransactionID]) {
		new.OnBranchConfirmed(event.ID)
	}))

	new.tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(onlyIfBootstrapped(t.TimeManager, func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		new.OnBranchCreated(event.ID)
	}))

	new.tangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(onlyIfBootstrapped(t.TimeManager, func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		new.OnBranchRejected(event.ID)
	}))

	new.tangle.TimeManager.Events.AcceptanceTimeUpdated.Attach(onlyIfBootstrapped(t.TimeManager, func(event *tangle.TimeUpdate) {
		new.OnAcceptanceTimeUpdated(event.ATT)
	}))

	return new
}

func onlyIfBootstrapped[E any](timeManager *tangle.TimeManager, handler func(event E)) *event.Closure[E] {
	return event.NewClosure(func(event E) {
		if !timeManager.Bootstrapped() {
			return
		}
		handler(event)
	})
}

// LoadSnapshot initiates the state and mana trees from a given snapshot.
func (m *Manager) LoadSnapshot(snapshot *ledger.Snapshot) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	for _, outputWithMetadata := range snapshot.OutputsWithMetadata {
		m.epochCommitmentFactory.storage.ledgerstateStorage.Store(outputWithMetadata).Release()
		err := m.epochCommitmentFactory.insertStateLeaf(outputWithMetadata.ID())
		if err != nil {
			m.log.Error(err)
		}
		err = m.epochCommitmentFactory.updateManaLeaf(outputWithMetadata, true)
		if err != nil {
			m.log.Error(err)
		}
	}
	for ei := snapshot.FullEpochIndex + 1; ei <= snapshot.DiffEpochIndex; ei++ {
		epochDiff := snapshot.EpochDiffs[ei]
		for _, spentOutputWithMetadata := range epochDiff.Spent() {
			spentOutputIDBytes := spentOutputWithMetadata.ID().Bytes()
			m.epochCommitmentFactory.storage.ledgerstateStorage.Delete(spentOutputIDBytes)
			if has, _ := m.epochCommitmentFactory.stateRootTree.Has(spentOutputIDBytes); !has {
				panic("epoch diff spends an output not contained in the ledger state")
			}
			_, err := m.epochCommitmentFactory.stateRootTree.Delete(spentOutputIDBytes)
			if err != nil {
				panic("could not delete leaf from state root tree")
			}
		}

		for _, createdOutputWithMetadata := range epochDiff.Created() {
			createdOutputIDBytes := createdOutputWithMetadata.ID().Bytes()
			m.epochCommitmentFactory.storage.ledgerstateStorage.Store(createdOutputWithMetadata).Release()
			_, err := m.epochCommitmentFactory.stateRootTree.Update(createdOutputIDBytes, createdOutputIDBytes)
			if err != nil {
				panic("could not update leaf of state root tree")
			}
		}
	}

	// The last committed epoch index corresponds to the last epoch diff stored in the snapshot.
	if err := m.epochCommitmentFactory.storage.setLatestCommittableEpochIndex(snapshot.DiffEpochIndex); err != nil {
		panic("could not set last committed epoch index")
	}

	// We assume as our earliest forking point the last epoch diff stored in the snapshot.
	if err := m.epochCommitmentFactory.storage.setLastConfirmedEpochIndex(snapshot.DiffEpochIndex); err != nil {
		panic("could not set last confirmed epoch index")
	}

	// We set it to the next epoch after snapshotted one. It will be updated upon first confirmed message will arrive.
	if err := m.epochCommitmentFactory.storage.setAcceptanceEpochIndex(snapshot.DiffEpochIndex + 1); err != nil {
		panic("could not set current epoch index")
	}

	m.epochCommitmentFactory.storage.ecRecordStorage.Store(snapshot.LatestECRecord).Release()
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func (m *Manager) GetLatestEC() (ecRecord *epoch.ECRecord, err error) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	latestCommittableEpoch, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	ecRecord = m.epochCommitmentFactory.loadECRecord(latestCommittableEpoch)
	if ecRecord == nil {
		err = errors.Errorf("could not get latest commitment")
	}
	return
}

// LatestConfirmedEpochIndex returns the latest epoch index that has been confirmed.
func (m *Manager) LatestConfirmedEpochIndex() (epoch.Index, error) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	return m.epochCommitmentFactory.storage.lastConfirmedEpochIndex()
}

// OnMessageConfirmed is the handler for message confirmed event.
func (m *Manager) OnMessageConfirmed(message *tangle.Message) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(message.IssuingTime())
	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("message %s confirmed with issuing time %s in already committed epoch %d", message.ID(), message.IssuingTime(), ei)
		return
	}
	err := m.epochCommitmentFactory.insertTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
		return
	}
	m.Events.TangleTreeInserted.Trigger(&TangleTreeUpdatedEvent{EI: ei, MessageID: message.ID()})

	nodeID := identity.NewID(message.IssuerPublicKey())
	err = m.epochCommitmentFactory.insertActivityLeaf(ei, nodeID)
	if err != nil && m.log != nil {
		m.log.Error(err)
		return
	}
	m.tangle.WeightProvider.Update(ei, nodeID)
	m.Events.ActivityTreeInserted.Trigger(&ActivityTreeUpdatedEvent{EI: ei, NodeID: nodeID})
}

// OnMessageOrphaned is the handler for message orphaned event.
func (m *Manager) OnMessageOrphaned(message *tangle.Message) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(message.IssuingTime())
	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("message %s orphaned with issuing time %s in already committed epoch %d", message.ID(), message.IssuingTime(), ei)
		return
	}
	err := m.epochCommitmentFactory.removeTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
	m.Events.TangleTreeRemoved.Trigger(&TangleTreeUpdatedEvent{EI: ei, MessageID: message.ID()})

	nodeID := identity.NewID(message.IssuerPublicKey())
	removed, err := m.epochCommitmentFactory.removeActivityLeaf(ei, nodeID)
	if err != nil && m.log != nil {
		m.log.Error(err)
		return
	}
	if removed {
		m.tangle.WeightProvider.Update(ei, nodeID)
		m.Events.ActivityTreeInserted.Trigger(&ActivityTreeUpdatedEvent{EI: ei, NodeID: nodeID})
	}

	transaction, isTransaction := message.Payload().(utxo.Transaction)
	if isTransaction {
		spent, created := m.resolveOutputs(transaction)
		m.epochCommitmentFactory.deleteDiffUTXOs(ei, created, spent)
		m.Events.UTXOTreeRemoved.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})
	}
}

// OnTransactionAccepted is the handler for transaction accepted event.
func (m *Manager) OnTransactionAccepted(event *ledger.TransactionAcceptedEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	txID := event.TransactionID

	var txInclusionTime time.Time
	m.tangle.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMeta *ledger.TransactionMetadata) {
		txInclusionTime = txMeta.InclusionTime()
	})
	txEpoch := epoch.IndexFromTime(txInclusionTime)

	if m.isEpochAlreadyCommitted(txEpoch) {
		m.log.Errorf("transaction %s confirmed with issuing time %s in already committed epoch %d", event.TransactionID, txInclusionTime, txEpoch)
		return
	}

	var spent, created []*ledger.OutputWithMetadata
	m.tangle.Ledger.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
		spent, created = m.resolveOutputs(tx)
	})
	if err := m.includeTransactionInEpoch(txID, txEpoch, spent, created); err != nil {
		m.log.Error(err)
	}
}

// OnTransactionInclusionUpdated is the handler for transaction inclusion updated event.
func (m *Manager) OnTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	oldEpoch := epoch.IndexFromTime(event.PreviousInclusionTime)
	newEpoch := epoch.IndexFromTime(event.InclusionTime)

	if oldEpoch == 0 || oldEpoch == newEpoch {
		return
	}

	if m.isEpochAlreadyCommitted(oldEpoch) || m.isEpochAlreadyCommitted(newEpoch) {
		m.log.Errorf("inclusion time of transaction changed for already committed epoch: previous EI %d, new EI %d", oldEpoch, newEpoch)
		return
	}

	txID := event.TransactionID

	var spent, created []*ledger.OutputWithMetadata
	m.tangle.Ledger.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
		spent, created = m.resolveOutputs(tx)
	})

	if err := m.removeTransactionFromEpoch(txID, oldEpoch, spent, created); err != nil {
		m.log.Error(err)
	}

	if err := m.includeTransactionInEpoch(txID, newEpoch, spent, created); err != nil {
		m.log.Error(err)
	}
}

// OnBranchConfirmed is the handler for branch confirmed event.
func (m *Manager) OnBranchConfirmed(branchID utxo.TransactionID) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()
	ei := m.getBranchEI(branchID, true)

	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("branch confirmed in already committed epoch %d", ei)
		return
	}
	m.decreasePendingConflictCounter(ei)
}

// OnBranchCreated is the handler for branch created event.
func (m *Manager) OnBranchCreated(branchID utxo.TransactionID) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getBranchEI(branchID, false)

	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("branch created in already committed epoch %d", ei)
		return
	}
	m.increasePendingConflictCounter(ei)
}

// OnBranchRejected is the handler for branch created event.
func (m *Manager) OnBranchRejected(branchID utxo.TransactionID) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getBranchEI(branchID, true)

	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("branch rejected in already committed epoch %d", ei)
		return
	}
	m.decreasePendingConflictCounter(ei)
}

// OnAcceptanceTimeUpdated is the handler for time updated event.
func (m *Manager) OnAcceptanceTimeUpdated(newTime time.Time) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(newTime)
	currentEpochIndex, err := m.epochCommitmentFactory.storage.acceptanceEpochIndex()
	if err != nil {
		m.log.Error(errors.Wrap(err, "could not get current epoch index"))
		return
	}
	if ei > currentEpochIndex {
		err = m.epochCommitmentFactory.storage.setAcceptanceEpochIndex(ei)
		if err != nil {
			m.log.Error(errors.Wrap(err, "could not set current epoch index"))
			return
		}
		m.moveLatestCommittableEpoch(ei)
	}
}

// PendingConflictsCount returns the current value of pendingConflictsCount.
func (m *Manager) PendingConflictsCount(ei epoch.Index) (pendingConflictsCount uint64) {
	return m.pendingConflictsCounters[ei]
}

// PendingConflictsCountAll returns the current value of pendingConflictsCount per epoch.
func (m *Manager) PendingConflictsCountAll() (pendingConflicts map[epoch.Index]uint64) {
	pendingConflicts = make(map[epoch.Index]uint64, len(m.pendingConflictsCounters))
	for k, v := range m.pendingConflictsCounters {
		pendingConflicts[k] = v
	}
	return pendingConflicts
}

// Shutdown shuts down the manager's permanent storagee.
func (m *Manager) Shutdown() {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	m.epochCommitmentFactory.storage.shutdown()
}

func (m *Manager) decreasePendingConflictCounter(ei epoch.Index) {
	m.pendingConflictsCounters[ei]--
	if m.pendingConflictsCounters[ei] == 0 {
		m.moveLatestCommittableEpoch(ei)
	}
}

func (m *Manager) increasePendingConflictCounter(ei epoch.Index) {
	m.pendingConflictsCounters[ei]++
}

func (m *Manager) includeTransactionInEpoch(txID utxo.TransactionID, ei epoch.Index, spent, created []*ledger.OutputWithMetadata) (err error) {
	if err := m.epochCommitmentFactory.insertStateMutationLeaf(ei, txID); err != nil {
		return err
	}
	m.epochCommitmentFactory.storeDiffUTXOs(ei, spent, created)

	m.Events.StateMutationTreeInserted.Trigger(&StateMutationTreeUpdatedEvent{TransactionID: txID})
	m.Events.UTXOTreeInserted.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})

	return nil
}

func (m *Manager) removeTransactionFromEpoch(txID utxo.TransactionID, ei epoch.Index, spent, created []*ledger.OutputWithMetadata) (err error) {
	if err := m.epochCommitmentFactory.removeStateMutationLeaf(ei, txID); err != nil {
		return err
	}
	m.epochCommitmentFactory.deleteDiffUTXOs(ei, spent, created)

	m.Events.StateMutationTreeRemoved.Trigger(&StateMutationTreeUpdatedEvent{TransactionID: txID})
	m.Events.UTXOTreeRemoved.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})

	return nil
}

// isCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) isCommittable(ei epoch.Index) bool {
	return m.isOldEnough(ei) && m.allPastConflictsAreResolved(ei)
}

func (m *Manager) allPastConflictsAreResolved(ei epoch.Index) (conflictsResolved bool) {
	lastEI, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	if err != nil {
		return false
	}
	// epoch is not committable if there are any not resolved conflicts in this and past epochs
	for index := lastEI; index <= ei; index++ {
		if m.pendingConflictsCounters[index] != 0 {
			return false
		}
	}
	return true
}

func (m *Manager) isOldEnough(ei epoch.Index, issuingTime ...time.Time) (oldEnough bool) {
	t := ei.EndTime()
	currentATT := m.tangle.TimeManager.ATT()
	if len(issuingTime) > 0 && issuingTime[0].After(currentATT) {
		currentATT = issuingTime[0]
	}

	diff := currentATT.Sub(t)
	if diff < m.options.MinCommittableEpochAge {
		return false
	}
	return true
}

func (m *Manager) getBranchEI(branchID utxo.TransactionID, earliestAttachmentMustBeBooked bool) (ei epoch.Index) {
	earliestAttachment := m.tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(branchID), earliestAttachmentMustBeBooked)
	ei = epoch.IndexFromTime(earliestAttachment.IssuingTime())
	return
}

func (m *Manager) isEpochAlreadyCommitted(ei epoch.Index) bool {
	latestCommittable, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	if err != nil {
		m.log.Errorf("could not get the latest committed epoch: %v", err)
		return false
	}
	return ei <= latestCommittable
}

func (m *Manager) resolveOutputs(tx utxo.Transaction) (spentOutputsWithMetadata, createdOutputsWithMetadata []*ledger.OutputWithMetadata) {
	spentOutputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	createdOutputsWithMetadata = make([]*ledger.OutputWithMetadata, 0)
	var spentOutputIDs utxo.OutputIDs
	var createdOutputs []utxo.Output

	spentOutputIDs = m.tangle.Ledger.Utils.ResolveInputs(tx.Inputs())
	createdOutputs = tx.(*devnetvm.Transaction).Essence().Outputs().UTXOOutputs()

	for it := spentOutputIDs.Iterator(); it.HasNext(); {
		spentOutputID := it.Next()
		m.tangle.Ledger.Storage.CachedOutput(spentOutputID).Consume(func(spentOutput utxo.Output) {
			m.tangle.Ledger.Storage.CachedOutputMetadata(spentOutputID).Consume(func(spentOutputMetadata *ledger.OutputMetadata) {
				spentOutputsWithMetadata = append(spentOutputsWithMetadata, ledger.NewOutputWithMetadata(spentOutputID, spentOutput, spentOutputMetadata.CreationTime(), spentOutputMetadata.ConsensusManaPledgeID(), spentOutputMetadata.AccessManaPledgeID()))
			})
		})
	}

	for _, createdOutput := range createdOutputs {
		createdOutputID := createdOutput.ID()
		m.tangle.Ledger.Storage.CachedOutputMetadata(createdOutputID).Consume(func(createdOutputMetadata *ledger.OutputMetadata) {
			createdOutputsWithMetadata = append(createdOutputsWithMetadata, ledger.NewOutputWithMetadata(createdOutputID, createdOutput, createdOutputMetadata.CreationTime(), createdOutputMetadata.ConsensusManaPledgeID(), createdOutputMetadata.AccessManaPledgeID()))
		})
	}

	return
}

func (m *Manager) triggerManaVectorUpdate(ei epoch.Index) {
	epochForManaVector := ei - epoch.Index(m.options.ManaEpochDelay)
	if epochForManaVector < 1 {
		return
	}
	spent, created := m.epochCommitmentFactory.loadDiffUTXOs(epochForManaVector)
	m.Events.ManaVectorUpdate.Trigger(&ManaVectorUpdateEvent{
		EI:               ei,
		EpochDiffCreated: created,
		EpochDiffSpent:   spent,
	})
}

func (m *Manager) moveLatestCommittableEpoch(currentEpoch epoch.Index) {
	latestCommittable, err := m.epochCommitmentFactory.storage.latestCommittableEpochIndex()
	if err != nil {
		m.log.Errorf("could not obtain last committed epoch index: %v", err)
		return
	}
	for ei := latestCommittable + 1; ei <= currentEpoch; ei++ {
		if !m.isCommittable(ei) {
			break
		}

		// reads the roots and store the ec
		// rolls the state trees
		ecRecord, ecRecordErr := m.epochCommitmentFactory.ecRecord(ei)
		if ecRecordErr != nil {
			m.log.Errorf("could not update commitments for epoch %d: %v", ei, ecRecordErr)
			return
		}

		if err = m.epochCommitmentFactory.storage.setLatestCommittableEpochIndex(ei); err != nil {
			m.log.Errorf("could not set last committed epoch: %v", err)
			return
		}

		m.Events.EpochCommittable.Trigger(&EpochCommittableEvent{EI: ei, ECRecord: ecRecord})
		m.triggerManaVectorUpdate(ei)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// ManagerOption represents the return type of the optional config parameters of the notarization manager.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container of all the config parameters of the notarization manager.
type ManagerOptions struct {
	MinCommittableEpochAge time.Duration
	ManaEpochDelay         uint
	Log                    *logger.Logger
}

// MinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func MinCommittableEpochAge(d time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.MinCommittableEpochAge = d
	}
}

// ManaDelay specifies the epoch offset for mana vector from the last committable epoch.
func ManaDelay(d uint) ManagerOption {
	return func(options *ManagerOptions) {
		options.ManaEpochDelay = d
	}
}

// Log provides the logger.
func Log(log *logger.Logger) ManagerOption {
	return func(options *ManagerOptions) {
		options.Log = log
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the existing events of a notarization manager.
type Events struct {
	// EpochCommittable is an event that gets triggered whenever an epoch commitment is committable.
	EpochCommittable *event.Event[*EpochCommittableEvent]
	// ManaVectorUpdate is an event that gets triggered when we move to the next epoch and mana vector should be updated.
	ManaVectorUpdate *event.Event[*ManaVectorUpdateEvent]
	// TangleTreeInserted is an event that gets triggered when a Message is inserted into the Tangle smt.
	TangleTreeInserted *event.Event[*TangleTreeUpdatedEvent]
	// TangleTreeRemoved is an event that gets triggered when a Message is removed from Tangle smt.
	TangleTreeRemoved *event.Event[*TangleTreeUpdatedEvent]
	// StateMutationTreeInserted is an event that gets triggered when a transaction is inserted into the state mutation smt.
	StateMutationTreeInserted *event.Event[*StateMutationTreeUpdatedEvent]
	// StateMutationTreeRemoved is an event that gets triggered when a transaction is removed from state mutation smt.
	StateMutationTreeRemoved *event.Event[*StateMutationTreeUpdatedEvent]
	// UTXOTreeInserted is an event that gets triggered when UTXOs are stored into the UTXO smt.
	UTXOTreeInserted *event.Event[*UTXOUpdatedEvent]
	// UTXOTreeRemoved is an event that gets triggered when UTXOs are removed from the UTXO smt.
	UTXOTreeRemoved *event.Event[*UTXOUpdatedEvent]
	// ActivityTreeInserted is an event that gets triggered when nodeID is added to the activity tree.
	ActivityTreeInserted *event.Event[*ActivityTreeUpdatedEvent]
	// ActivityTreeRemoved is an event that gets triggered when nodeID is removed from activity tree.
	ActivityTreeRemoved *event.Event[*ActivityTreeUpdatedEvent]
}

// TangleTreeUpdatedEvent is a container that acts as a dictionary for the TangleTree inserted/removed event related parameters.
type TangleTreeUpdatedEvent struct {
	// EI is the index of the message.
	EI epoch.Index
	// MessageID is the messageID that inserted/removed to/from the tangle smt.
	MessageID tangle.MessageID
}

// StateMutationTreeUpdatedEvent is a container that acts as a dictionary for the State mutation tree inserted/removed event related parameters.
type StateMutationTreeUpdatedEvent struct {
	// EI is the index of the transaction.
	EI epoch.Index
	// TransactionID is the transaction ID that inserted/removed to/from the state mutation smt.
	TransactionID utxo.TransactionID
}

// UTXOUpdatedEvent is a container that acts as a dictionary for the UTXO update event related parameters.
type UTXOUpdatedEvent struct {
	// EI is the index of updated UTXO.
	EI epoch.Index
	// Created are the outputs created in a transaction.
	Created []*ledger.OutputWithMetadata
	// Spent are outputs that is spent in a transaction.
	Spent []*ledger.OutputWithMetadata
}

// EpochCommittableEvent is a container that acts as a dictionary for the EpochCommittable event related parameters.
type EpochCommittableEvent struct {
	// EI is the index of committable epoch.
	EI epoch.Index
	// ECRecord is the ec root of committable epoch.
	ECRecord *epoch.ECRecord
}

// ManaVectorUpdateEvent is a container that acts as a dictionary for the EpochCommittable event related parameters.
type ManaVectorUpdateEvent struct {
	// EI is the index of committable epoch.
	EI               epoch.Index
	EpochDiffCreated []*ledger.OutputWithMetadata
	EpochDiffSpent   []*ledger.OutputWithMetadata
}

// ActivityTreeUpdatedEvent is a container that acts as a dictionary for the ActivityTree inserted/removed event related parameters.
type ActivityTreeUpdatedEvent struct {
	// EI is the index of the message.
	EI epoch.Index
	// NodeID is the issuer nodeID.
	NodeID identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
