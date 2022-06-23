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
	epochManager                *EpochManager
	epochCommitmentFactory      *EpochCommitmentFactory
	epochCommitmentFactoryMutex sync.RWMutex
	options                     *ManagerOptions
	pendingConflictsCounters    map[epoch.Index]uint64
	log                         *logger.Logger
	Events                      *Events
}

// NewManager creates and returns a new notarization manager.
func NewManager(epochManager *EpochManager, epochCommitmentFactory *EpochCommitmentFactory, t *tangle.Tangle, opts ...ManagerOption) (new *Manager) {
	options := &ManagerOptions{
		MinCommittableEpochAge: defaultMinEpochCommittableAge,
		Log:                    nil,
	}

	for _, option := range opts {
		option(options)
	}

	new = &Manager{
		tangle:                   t,
		epochManager:             epochManager,
		epochCommitmentFactory:   epochCommitmentFactory,
		pendingConflictsCounters: make(map[epoch.Index]uint64),
		log:                      options.Log,
		options:                  options,
		Events: &Events{
			EpochCommitted: event.New[*EpochCommittedEvent](),
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
				m.log.Error(err)
			}
		}

		for _, createdOutputWithMetadata := range epochDiff.Created() {
			createdOutputIDBytes := createdOutputWithMetadata.ID().Bytes()
			m.epochCommitmentFactory.storage.ledgerstateStorage.Store(createdOutputWithMetadata).Release()
			_, err := m.epochCommitmentFactory.stateRootTree.Update(createdOutputIDBytes, createdOutputIDBytes)
			if err != nil {
				m.log.Error(err)
			}
		}
	}

	// Our ledgerstate is aligned with the last committed epoch, which is the same as the last epoch in the snapshot.
	if err := m.epochCommitmentFactory.storage.SetFullEpochIndex(snapshot.DiffEpochIndex); err != nil {
		m.log.Error(err)
	}

	if err := m.epochCommitmentFactory.storage.SetDiffEpochIndex(snapshot.DiffEpochIndex); err != nil {
		m.log.Error(err)
	}

	// The last committed epoch index corresponds to the last epoch diff stored in the snapshot.
	if err := m.epochCommitmentFactory.storage.SetLastCommittedEpochIndex(snapshot.DiffEpochIndex); err != nil {
		m.log.Error(err)
	}

	// We assume as our earliest forking point the last epoch diff stored in the snapshot.
	if err := m.epochCommitmentFactory.storage.SetLastConfirmedEpochIndex(snapshot.DiffEpochIndex); err != nil {
		m.log.Error(err)
	}

	m.epochCommitmentFactory.storage.ecRecordStorage.Store(snapshot.LatestECRecord).Release()
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func (m *Manager) GetLatestEC() (ecRecord *epoch.ECRecord, err error) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	lastCommittedEpoch, latestCommittableEpoch, lastCommittableEpochErr := m.latestCommittableEpoch()
	if lastCommittableEpochErr != nil {
		return nil, errors.Wrap(lastCommittableEpochErr, "could not get last committable epoch")
	}

	if updateErr := m.updateCommitmentsUpToLatestCommittableEpoch(lastCommittedEpoch, latestCommittableEpoch); updateErr != nil {
		err = errors.Wrap(updateErr, "could not update commitments up to latest committable epoch")
		return nil, err
	}

	if ecRecord, err = m.epochCommitmentFactory.ecRecord(latestCommittableEpoch); err != nil {
		return nil, errors.Wrap(err, "could not get latest epoch commitment")
	}

	if err := m.epochCommitmentFactory.storage.SetLastCommittedEpochIndex(latestCommittableEpoch); err != nil {
		return nil, errors.Wrap(err, "could not set last committed epoch")
	}

	m.Events.EpochCommitted.Trigger(&EpochCommittedEvent{EI: latestCommittableEpoch})

	return
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

	ei := m.epochManager.TimeToEI(message.IssuingTime())
	if m.isEpochAlreadyComitted(ei) {
		m.log.Errorf("message confirmed in already committed epoch %d", ei)
	}
	err := m.epochCommitmentFactory.insertTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
}

// OnMessageOrphaned is the handler for message orphaned event.
func (m *Manager) OnMessageOrphaned(message *tangle.Message) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.epochManager.TimeToEI(message.IssuingTime())
	if m.isEpochAlreadyComitted(ei) {
		m.log.Errorf("message orphaned in already committed epoch %d", ei)
	}
	err := m.epochCommitmentFactory.removeTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
	transaction, isTransaction := message.Payload().(utxo.Transaction)
	if isTransaction {
		spent, created := m.resolveOutputs(transaction)
		m.epochCommitmentFactory.deleteDiffUTXOs(ei, created, spent)
	}
}

func (m *Manager) OnTransactionConfirmed(event *ledger.TransactionConfirmedEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	var spent, created []*ledger.OutputWithMetadata
	m.tangle.Ledger.Storage.CachedTransaction(event.TransactionID).Consume(func(tx utxo.Transaction) {
		spent, created = m.resolveOutputs(tx)
	})

	txID := event.TransactionID

	var txEpoch epoch.Index
	m.tangle.Ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMeta *ledger.TransactionMetadata) {
		txEpoch = m.epochManager.TimeToEI(txMeta.InclusionTime())
	})

	if err := m.includeTransactionInEpoch(txID, txEpoch, spent, created); err != nil {
		m.log.Error(err)
	}
}

// OnTransactionInclusionUpdated is the handler for transaction inclusion updated event.
func (m *Manager) OnTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	oldEpoch := m.epochManager.TimeToEI(event.PreviousInclusionTime)
	newEpoch := m.epochManager.TimeToEI(event.InclusionTime)

	if oldEpoch == 0 || oldEpoch == newEpoch {
		return
	}

	if m.isEpochAlreadyComitted(oldEpoch) || m.isEpochAlreadyComitted(newEpoch) {
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
	m.pendingConflictsCounters[ei]--
}

// OnBranchCreated is the handler for branch created event.
func (m *Manager) OnBranchCreated(branchID utxo.TransactionID) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getBranchEI(branchID, false)
	m.pendingConflictsCounters[ei]++
}

// OnBranchRejected is the handler for branch created event.
func (m *Manager) OnBranchRejected(branchID utxo.TransactionID) {
	m.epochCommitmentFactoryMutex.Lock()
	defer m.epochCommitmentFactoryMutex.Unlock()

	ei := m.getBranchEI(branchID, true)
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

	m.epochCommitmentFactory.storeDiffUTXOs(ei, spent, created)

	return nil
}

func (m *Manager) removeTransactionFromEpoch(txID utxo.TransactionID, ei epoch.Index, spent, created []*ledger.OutputWithMetadata) (err error) {
	if err := m.epochCommitmentFactory.removeStateMutationLeaf(ei, txID); err != nil {
		return err
	}

	m.epochCommitmentFactory.deleteDiffUTXOs(ei, spent, created)

	return nil
}

func (m *Manager) latestCommittableEpoch() (lastCommittedEpoch, latestCommittableEpoch epoch.Index, err error) {
	currentEpoch := m.epochManager.TimeToEI(time.Now())

	lastCommittedEpoch, lastCommittedEpochErr := m.epochCommitmentFactory.storage.LastCommittedEpochIndex()
	if lastCommittedEpochErr != nil {
		err = errors.Wrap(lastCommittedEpochErr, "could not obtain last committed epoch index")
		return
	}

	for ei := lastCommittedEpoch; ei < currentEpoch; ei++ {
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
	t := m.epochManager.EIToEndTime(ei)
	diff := time.Since(t)
	return m.pendingConflictsCounters[ei] == 0 && diff >= m.options.MinCommittableEpochAge
}

func (m *Manager) getBranchEI(branchID utxo.TransactionID, earliestAttachmentMustBeBooked bool) (ei epoch.Index) {
	earliestAttachment := m.tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(branchID), earliestAttachmentMustBeBooked)
	ei = m.epochManager.TimeToEI(earliestAttachment.IssuingTime())
	return
}

// updateCommitmentsUpToLatestCommittableEpoch updates the commitments to align with the latest committable epoch.
func (m *Manager) updateCommitmentsUpToLatestCommittableEpoch(lastCommitted, latestCommittable epoch.Index) (err error) {
	var ei epoch.Index
	for ei = lastCommitted + 1; ei < latestCommittable; ei++ {
		// read the roots and store the ec
		// roll the state trees
		if _, ecRecordErr := m.epochCommitmentFactory.ecRecord(ei); ecRecordErr != nil {
			err = errors.Wrapf(ecRecordErr, "could not update commitments for epoch %d", ei)
			return
		}

		// update last committed index
		if setLastCommittedEpochIndexErr := m.epochCommitmentFactory.storage.SetLastCommittedEpochIndex(ei); setLastCommittedEpochIndexErr != nil {
			err = errors.Wrap(setLastCommittedEpochIndexErr, "could not set last committed epoch")
			return
		}
	}

	return
}

func (m *Manager) isEpochAlreadyComitted(ei epoch.Index) bool {
	lastCommitted, _, err := m.latestCommittableEpoch()
	if err != nil {
		m.log.Errorf("could not determine latest committed epoch: %v", err)
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

func (m *Manager) outputIDsToOutputs(outputIDs utxo.OutputIDs) (outputsVm devnetvm.Outputs) {
	for it := outputIDs.Iterator(); it.HasNext(); {
		outputID := it.Next()
		m.tangle.Ledger.Storage.CachedOutput(outputID).Consume(func(out utxo.Output) {
			outputsVm = append(outputsVm, out.(devnetvm.Output))
		})
	}
	return
}

func (m *Manager) outputsToOutputIDs(outputs devnetvm.Outputs) (createdIDs utxo.OutputIDs) {
	for _, o := range outputs {
		createdIDs.Add(o.ID())
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
}

// EpochCommittedEvent is a container that acts as a dictionary for the EpochCommitted event related parameters.
type EpochCommittedEvent struct {
	// EI is the index of committable epoch.
	EI epoch.Index
}
