package notarization

import (
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

// Manager is the notarization manager.
type Manager struct {
	tangle                      *tangle.Tangle
	epochCommitmentFactory      *EpochCommitmentFactory
	epochCommitmentFactoryMutex sync.RWMutex
	lastCommittedEpoch          *epoch.ECRecord
	options                     *ManagerOptions
	pccMutex                    sync.RWMutex
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
			EpochCommitted:        event.New[*EpochCommittedEvent](),
			TangleTreeInserted:    event.New[*TangleTreeUpdatedEvent](),
			TangleTreeRemoved:     event.New[*TangleTreeUpdatedEvent](),
			StateMutationInserted: event.New[*StateMutationTreeUpdatedEvent](),
			StateMutationRemoved:  event.New[*StateMutationTreeUpdatedEvent](),
			UTXOInserted:          event.New[*UTXOUpdatedEvent](),
			UTXORemoved:           event.New[*UTXOUpdatedEvent](),
		},
	}

	new.tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(onlyIfBootstrapped(t.TimeManager, func(event *tangle.MessageConfirmedEvent) {
		new.OnMessageConfirmed(event.Message)
	}))

	new.tangle.ConfirmationOracle.Events().MessageOrphaned.Attach(onlyIfBootstrapped(t.TimeManager, func(event *tangle.MessageConfirmedEvent) {
		new.OnMessageOrphaned(event.Message)
	}))

	new.tangle.Ledger.Events.TransactionConfirmed.Attach(onlyIfBootstrapped(t.TimeManager, func(event *ledger.TransactionConfirmedEvent) {
		new.OnTransactionConfirmed(event)
	}))

	new.tangle.Ledger.Events.TransactionInclusionUpdated.Attach(onlyIfBootstrapped(t.TimeManager, func(event *ledger.TransactionInclusionUpdatedEvent) {
		new.OnTransactionInclusionUpdated(event)
	}))

	new.tangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(onlyIfBootstrapped(t.TimeManager, func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		new.OnBranchConfirmed(event.ID)
	}))

	new.tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(onlyIfBootstrapped(t.TimeManager, func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		new.OnBranchCreated(event.ID)
	}))

	new.tangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(onlyIfBootstrapped(t.TimeManager, func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		new.OnBranchRejected(event.ID)
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

	// Our ledgerstate is aligned with the last committed epoch, which is the same as the last epoch in the snapshot.
	if err := m.epochCommitmentFactory.storage.SetFullEpochIndex(snapshot.DiffEpochIndex); err != nil {
		panic("could not set full epoch index")
	}

	if err := m.epochCommitmentFactory.storage.SetDiffEpochIndex(snapshot.DiffEpochIndex); err != nil {
		panic("could not set diff epoch index")
	}

	// The last committed epoch index corresponds to the last epoch diff stored in the snapshot.
	if err := m.epochCommitmentFactory.storage.SetLastCommittedEpochIndex(snapshot.DiffEpochIndex); err != nil {
		panic("could not set last committed epoch index")
	}

	// We assume as our earliest forking point the last epoch diff stored in the snapshot.
	if err := m.epochCommitmentFactory.storage.SetLastConfirmedEpochIndex(snapshot.DiffEpochIndex); err != nil {
		panic("could not set last confirmed epoch index")
	}

	m.epochCommitmentFactory.storage.ecRecordStorage.Store(snapshot.LatestECRecord).Release()
}

// PendingConflictsCount returns the current value of pendingConflictsCount.
func (m *Manager) PendingConflictsCount(ei epoch.Index) uint64 {
	m.pccMutex.RLock()
	defer m.pccMutex.RUnlock()
	return m.pendingConflictsCounters[ei]
}

// PendingConflictsCountAll returns the current value of pendingConflictsCount per epoch.
func (m *Manager) PendingConflictsCountAll() map[epoch.Index]uint64 {
	m.pccMutex.RLock()
	defer m.pccMutex.RUnlock()
	duplicate := make(map[epoch.Index]uint64, len(m.pendingConflictsCounters))
	for k, v := range m.pendingConflictsCounters {
		duplicate[k] = v
	}
	return duplicate
}

// IsCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) IsCommittable(ei epoch.Index) bool {
	t := ei.EndTime()
	diff := time.Since(t)
	return m.pendingConflictsCounters[ei] == 0 && diff >= m.options.MinCommittableEpochAge
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func (m *Manager) GetLatestEC() (ecRecord *epoch.ECRecord, err error) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ecRecord, updateErr := m.updateCommitmentsToLatestCommittableEpoch()
	if updateErr != nil {
		return nil, errors.Wrap(updateErr, "could not update commitments up to latest committable epoch")
	}

	return
}

// TODO: double check if we need this function.
func (m *Manager) LastCommittedEpoch() (*epoch.ECRecord, error) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()
	ei, err := m.epochCommitmentFactory.storage.LastCommittedEpochIndex()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ecr, _, err := m.epochCommitmentFactory.ecRecord(ei)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return ecr, nil
}

func (m *Manager) LatestConfirmedEpochIndex() (epoch.Index, error) {
	m.epochCommitmentFactoryMutex.RLock()
	defer m.epochCommitmentFactoryMutex.RUnlock()

	return m.epochCommitmentFactory.storage.LastConfirmedEpochIndex()
}

// OnMessageConfirmed is the handler for message confirmed event.
func (m *Manager) OnMessageConfirmed(message *tangle.Message) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(message.IssuingTime())
	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("message confirmed in already committed epoch %d", ei)
		return
	}
	err := m.epochCommitmentFactory.insertTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
		return
	}
	m.Events.TangleTreeInserted.Trigger(&TangleTreeUpdatedEvent{EI: ei, MessageID: message.ID()})
}

// OnMessageOrphaned is the handler for message orphaned event.
func (m *Manager) OnMessageOrphaned(message *tangle.Message) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := epoch.IndexFromTime(message.IssuingTime())
	if m.isEpochAlreadyCommitted(ei) {
		m.log.Errorf("message orphaned in already committed epoch %d", ei)
		return
	}
	err := m.epochCommitmentFactory.removeTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
	m.Events.TangleTreeRemoved.Trigger(&TangleTreeUpdatedEvent{EI: ei, MessageID: message.ID()})

	transaction, isTransaction := message.Payload().(utxo.Transaction)
	if isTransaction {
		spent, created := m.resolveOutputs(transaction)
		m.epochCommitmentFactory.deleteDiffUTXOs(ei, created, spent)
		m.Events.UTXORemoved.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})
	}
}

func (m *Manager) OnTransactionConfirmed(event *ledger.TransactionConfirmedEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	txID := event.TransactionID

	var txEpoch epoch.Index
	var zeroInclusion bool
	m.tangle.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMeta *ledger.TransactionMetadata) {
		if txMeta.InclusionTime().IsZero() {
			zeroInclusion = true
			return
		}
		txEpoch = epoch.IndexFromTime(txMeta.InclusionTime())
	})

	if zeroInclusion {
		m.log.Error("transaction confirmed with zero inclusion time")
		return
	}

	if m.isEpochAlreadyCommitted(txEpoch) {
		m.log.Errorf("transaction confirmed in already committed epoch %d", txEpoch)
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

	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()
	m.pendingConflictsCounters[ei]--
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

	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()
	m.pendingConflictsCounters[ei]++
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

	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()
	m.pendingConflictsCounters[ei]--
}

func (m *Manager) Shutdown() {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	m.epochCommitmentFactory.storage.Shutdown()
}

func (m *Manager) includeTransactionInEpoch(txID utxo.TransactionID, ei epoch.Index, spent, created []*ledger.OutputWithMetadata) (err error) {
	if err := m.epochCommitmentFactory.insertStateMutationLeaf(ei, txID); err != nil {
		return err
	}
	m.Events.StateMutationInserted.Trigger(&StateMutationTreeUpdatedEvent{TransactionID: txID})

	m.epochCommitmentFactory.storeDiffUTXOs(ei, spent, created)
	m.Events.UTXOInserted.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})

	return nil
}

func (m *Manager) removeTransactionFromEpoch(txID utxo.TransactionID, ei epoch.Index, spent, created []*ledger.OutputWithMetadata) (err error) {
	if err := m.epochCommitmentFactory.removeStateMutationLeaf(ei, txID); err != nil {
		return err
	}
	m.Events.StateMutationRemoved.Trigger(&StateMutationTreeUpdatedEvent{TransactionID: txID})

	m.epochCommitmentFactory.deleteDiffUTXOs(ei, spent, created)
	m.Events.UTXORemoved.Trigger(&UTXOUpdatedEvent{EI: ei, Spent: spent, Created: created})

	return nil
}

func (m *Manager) latestCommittableEpoch() (lastCommittedEpoch, latestCommittableEpoch epoch.Index, err error) {
	currentEpoch := epoch.CurrentEpochIndex()

	lastCommittedEpoch, lastCommittedEpochErr := m.epochCommitmentFactory.storage.LastCommittedEpochIndex()
	if lastCommittedEpochErr != nil {
		err = errors.Wrap(lastCommittedEpochErr, "could not obtain last committed epoch index")
		return
	}

	latestCommittableEpoch = lastCommittedEpoch
	for ei := lastCommittedEpoch + 1; ei < currentEpoch; ei++ {
		if m.isCommittable(ei) {
			latestCommittableEpoch = ei
			continue
		}
		break
	}

	if latestCommittableEpoch == currentEpoch {
		err = errors.Errorf("latestCommittableEpoch cannot be current epoch")
		return
	}

	return lastCommittedEpoch, latestCommittableEpoch, nil
}

// isCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) isCommittable(ei epoch.Index) bool {
	t := ei.EndTime()
	diff := m.tangle.TimeManager.ATT().Sub(t)
	return m.pendingConflictsCounters[ei] == 0 && diff >= m.options.MinCommittableEpochAge
}

func (m *Manager) getBranchEI(branchID utxo.TransactionID, earliestAttachmentMustBeBooked bool) (ei epoch.Index) {
	earliestAttachment := m.tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(branchID), earliestAttachmentMustBeBooked)
	ei = epoch.IndexFromTime(earliestAttachment.IssuingTime())
	return
}

// updateCommitmentsToLatestCommittableEpoch updates the commitments to align with the latest committable epoch.
func (m *Manager) updateCommitmentsToLatestCommittableEpoch() (ecRecord *epoch.ECRecord, err error) {
	lastCommitted, latestCommittable, lastCommittableEpochErr := m.latestCommittableEpoch()
	if lastCommittableEpochErr != nil {
		return nil, errors.Wrap(lastCommittableEpochErr, "could not get last committable epoch")
	}

	for ei := lastCommitted; ei <= latestCommittable; ei++ {
		var isNew bool
		var ecRecordErr error

		// reads the roots and store the ec
		// rolls the state trees
		ecRecord, isNew, ecRecordErr = m.epochCommitmentFactory.ecRecord(ei)
		if ecRecordErr != nil {
			err = errors.Wrapf(ecRecordErr, "could not update commitments for epoch %d", ei)
			return nil, err
		}

		if isNew {
			if err := m.epochCommitmentFactory.storage.SetLastCommittedEpochIndex(ei); err != nil {
				return nil, errors.Wrap(err, "could not set last committed epoch")
			}

			m.Events.EpochCommitted.Trigger(&EpochCommittedEvent{EI: ei})
		}
	}

	return ecRecord, nil
}

func (m *Manager) isEpochAlreadyCommitted(ei epoch.Index) bool {
	lastCommitted, _, err := m.latestCommittableEpoch()
	if err != nil {
		m.log.Errorf("could not determine latest committed epoch: %v", err)
		return false
	}
	return ei <= lastCommitted
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
				spentOutputsWithMetadata = append(spentOutputsWithMetadata, ledger.NewOutputWithMetadata(spentOutputID, spentOutput, spentOutputMetadata))
			})
		})
	}

	for _, createdOutput := range createdOutputs {
		createdOutputID := createdOutput.ID()
		m.tangle.Ledger.Storage.CachedOutputMetadata(createdOutputID).Consume(func(createdOutputMetadata *ledger.OutputMetadata) {
			createdOutputsWithMetadata = append(createdOutputsWithMetadata, ledger.NewOutputWithMetadata(createdOutputID, createdOutput, createdOutputMetadata))
		})
	}

	return
}

// ManagerOption represents the return type of the optional config parameters of the notarization manager.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container of all the config parameters of the notarization manager.
type ManagerOptions struct {
	MinCommittableEpochAge time.Duration
	Log                    *logger.Logger
}

// MinCommittableEpochAge specifies how old an epoch has to be for it to be committable.
func MinCommittableEpochAge(d time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.MinCommittableEpochAge = d
	}
}

// Log provides the logger.
func Log(log *logger.Logger) ManagerOption {
	return func(options *ManagerOptions) {
		options.Log = log
	}
}

// Events is a container that acts as a dictionary for the existing events of a notarization manager.
type Events struct {
	// EpochCommitted is an event that gets triggered whenever an epoch commitment is committable.
	EpochCommitted *event.Event[*EpochCommittedEvent]
	// TangleTreeInserted is an event that gets triggered when a Message is inserted to Tangle smt.
	TangleTreeInserted *event.Event[*TangleTreeUpdatedEvent]
	// TangleTreeRemoved is an event that gets triggered when a Message is removed to Tangle smt.
	TangleTreeRemoved *event.Event[*TangleTreeUpdatedEvent]
	// StateMutationInserted is an event that gets triggered when a transaction is inserted to state mutation smt.
	StateMutationInserted *event.Event[*StateMutationTreeUpdatedEvent]
	// StateMutationRemoved is an event that gets triggered when a transaction is removed to state mutation smt.
	StateMutationRemoved *event.Event[*StateMutationTreeUpdatedEvent]
	// UTXOInserted is an event that gets triggered when UTXOs are stored.
	UTXOInserted *event.Event[*UTXOUpdatedEvent]
	// UTXORemoved is an event that gets triggered when UTXOs are removed.
	UTXORemoved *event.Event[*UTXOUpdatedEvent]
}

// EpochCommittedEvent is a container that acts as a dictionary for the EpochCommitted event related parameters.
type EpochCommittedEvent struct {
	// EI is the index of committable epoch.
	EI epoch.Index
	// ECRecord is the ecrecord of committed epoch.
	ECRecord *epoch.ECRecord
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
