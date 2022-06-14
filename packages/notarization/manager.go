package notarization

import (
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"

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
	pendingConflictsCount       map[epoch.Index]uint64
	pccMutex                    sync.RWMutex
	log                         *logger.Logger
	Events                      *Events
}

// NewManager creates and returns a new notarization manager.
func NewManager(epochManager *EpochManager, epochCommitmentFactory *EpochCommitmentFactory, tangle *tangle.Tangle, opts ...ManagerOption) *Manager {
	options := &ManagerOptions{
		MinCommittableEpochAge: defaultMinEpochCommittableAge,
		Log:                    nil,
	}
	for _, option := range opts {
		option(options)
	}
	return &Manager{
		tangle:                 tangle,
		epochManager:           epochManager,
		epochCommitmentFactory: epochCommitmentFactory,
		pendingConflictsCount:  make(map[epoch.Index]uint64),
		log:                    options.Log,
		options:                options,
		Events: &Events{
			EpochCommitted: event.New[*EpochCommittedEvent](),
		},
	}
}

// LoadSnapshot initiates the state and mana trees from a given snapshot.
func (m *Manager) LoadSnapshot(snapshot *ledger.Snapshot) {
	for _, outputWithMetadata := range snapshot.OutputsWithMetadata {
		m.epochCommitmentFactory.storage.ledgerstateStorage.Store(outputWithMetadata).Release()
		outputIDBytes := outputWithMetadata.ID().Bytes()
		_, err := m.epochCommitmentFactory.stateRootTree.Update(outputIDBytes, outputIDBytes)
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
			m.epochCommitmentFactory.storage.ledgerstateStorage.Store(createdOutputWithMetadata)
			_, err := m.epochCommitmentFactory.stateRootTree.Update(createdOutputIDBytes, createdOutputIDBytes)
			if err != nil {
				m.log.Error(err)
			}
		}
	}

	// TODO: mana root

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

// PendingConflictsCount returns the current value of pendingConflictsCount.
func (m *Manager) PendingConflictsCount(ei epoch.Index) uint64 {
	m.pccMutex.RLock()
	defer m.pccMutex.RUnlock()
	return m.pendingConflictsCount[ei]
}

// IsCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) IsCommittable(ei epoch.Index) bool {
	t := m.epochManager.EIToEndTime(ei)
	diff := time.Since(t)
	return m.PendingConflictsCount(ei) == 0 && diff >= m.options.MinCommittableEpochAge
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
	return m.epochCommitmentFactory.storage.LastConfirmedEpochIndex()
}

// OnMessageConfirmed is the handler for message confirmed event.
func (m *Manager) OnMessageConfirmed(message *tangle.Message) {
	ei := m.epochManager.TimeToEI(message.IssuingTime())
	err := m.epochCommitmentFactory.InsertTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
}

// OnMessageOrphaned is the handler for message orphaned event.
func (m *Manager) OnMessageOrphaned(message *tangle.Message) {
	ei := m.epochManager.TimeToEI(message.IssuingTime())
	err := m.epochCommitmentFactory.RemoveTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
	// TODO: think about transaction case.
}

// OnTransactionConfirmed is the handler for transaction confirmed event.
func (m *Manager) OnTransactionConfirmed(tx *devnetvm.Transaction) {
	earliestAttachment := m.tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
	ei := m.epochManager.TimeToEI(earliestAttachment.IssuingTime())
	err := m.epochCommitmentFactory.InsertStateMutationLeaf(ei, tx.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
	m.storeTXDiff(ei, tx)
}

// OnTransactionInclusionUpdated is the handler for transaction inclusion updated event.
func (m *Manager) OnTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	prevEpoch := m.epochManager.TimeToEI(event.PreviousInclusionTime)
	newEpoch := m.epochManager.TimeToEI(event.InclusionTime)

	if prevEpoch == newEpoch {
		return
	}

	err := m.epochCommitmentFactory.RemoveStateMutationLeaf(prevEpoch, event.TransactionID)
	if err != nil {
		m.log.Error(err)
	}
	err = m.epochCommitmentFactory.InsertStateMutationLeaf(newEpoch, event.TransactionID)
	if err != nil {
		m.log.Error(err)
	}
	// TODO: propagate updates to future epochs
	// TODO: update state tree
}

// OnBranchConfirmed is the handler for branch confirmed event.
func (m *Manager) OnBranchConfirmed(branchID utxo.TransactionID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei]--
}

// OnBranchCreated is the handler for branch created event.
func (m *Manager) OnBranchCreated(branchID utxo.TransactionID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei]++
}

// OnBranchRejected is the handler for branch created event.
func (m *Manager) OnBranchRejected(branchID utxo.TransactionID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei]--
}

func (m *Manager) latestCommittableEpoch() (lastCommittedEpoch, latestCommittableEpoch epoch.Index, err error) {
	currentEpoch := m.epochManager.TimeToEI(time.Now())

	lastCommittedEpoch, lastCommittedEpochErr := m.epochCommitmentFactory.storage.LastCommittedEpochIndex()
	if lastCommittedEpochErr != nil {
		err = errors.Wrap(lastCommittedEpochErr, "could not obtain last committed epoch index")
		return
	}

	for ei := lastCommittedEpoch; ei < currentEpoch; ei++ {
		if m.IsCommittable(ei) {
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

func (m *Manager) storeTXDiff(ei epoch.Index, tx *devnetvm.Transaction) {
	outputsSpent := m.tangle.Ledger.Utils.ResolveInputs(tx.Inputs())
	outputsCreated := tx.Essence().Outputs()

	// store outputs in the commitment diff storage
	m.epochCommitmentFactory.storeDiffUTXOs(ei, outputsSpent, outputsCreated)
}

func (m *Manager) getBranchEI(branchID utxo.TransactionID) (ei epoch.Index) {
	m.tangle.Ledger.Storage.CachedTransaction(branchID).Consume(func(tx utxo.Transaction) {
		earliestAttachment := m.tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
		ei = m.epochManager.TimeToEI(earliestAttachment.IssuingTime())
	})
	return
}

// GetBlockInclusionProof gets the proof of the inclusion (acceptance) of a block.
func (m *Manager) GetBlockInclusionProof(blockID tangle.MessageID) (*CommitmentProof, error) {
	var ei epoch.Index
	m.tangle.Storage.Message(blockID).Consume(func(block *tangle.Message) {
		t := block.IssuingTime()
		ei = m.epochManager.TimeToEI(t)
	})
	proof, err := m.epochCommitmentFactory.ProofTangleRoot(ei, blockID)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// GetTransactionInclusionProof gets the proof of the inclusion (acceptance) of a transaction.
func (m *Manager) GetTransactionInclusionProof(transactionID utxo.TransactionID) (*CommitmentProof, error) {
	var ei epoch.Index
	m.tangle.Ledger.Storage.CachedTransaction(transactionID).Consume(func(tx utxo.Transaction) {
		t := tx.(*devnetvm.Transaction).Essence().Timestamp()
		ei = m.epochManager.TimeToEI(t)
	})
	proof, err := m.epochCommitmentFactory.ProofStateMutationRoot(ei, transactionID)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

// updateCommitmentsUpToLatestCommittableEpoch updates the commitments to align with the latest committable epoch.
func (m *Manager) updateCommitmentsUpToLatestCommittableEpoch(lastCommitted, latestCommittable epoch.Index) (err error) {
	fmt.Println("\t>> updateCommitmentsUpToLatestCommittableEpoch", lastCommitted, latestCommittable)

	var ei epoch.Index
	for ei = lastCommitted + 1; ei < latestCommittable; ei++ {
		// read the roots and store the ec
		// roll the state trees
		if _, ecRecordErr := m.epochCommitmentFactory.ecRecord(ei); ecRecordErr != nil {
			err = errors.Wrapf(ecRecordErr, "could not update committments for epoch %d", ei)
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
