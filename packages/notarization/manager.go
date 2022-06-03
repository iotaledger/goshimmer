package notarization

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/logger"
)

const (
	minEpochCommittableDuration = 24 * time.Minute
)

// Manager is the notarization manager.
type Manager struct {
	tangle                 *tangle.Tangle
	epochManager           *EpochManager
	epochCommitmentFactory *EpochCommitmentFactory
	options                *ManagerOptions
	pendingConflictsCount  map[epoch.EI]uint64
	pccMutex               sync.RWMutex
	log                    *logger.Logger
	Events                 *Events

	// lastCommittedEpoch is the last epoch that was committed, and the state tree is built upon this epoch.
	lastCommittedEpoch epoch.EI
}

// NewManager creates and returns a new notarization manager.
func NewManager(epochManager *EpochManager, epochCommitmentFactory *EpochCommitmentFactory, tangle *tangle.Tangle, opts ...ManagerOption) *Manager {
	options := &ManagerOptions{
		MinCommittableEpochAge: minEpochCommittableDuration,
		Log:                    nil,
	}
	for _, option := range opts {
		option(options)
	}
	return &Manager{
		tangle:                 tangle,
		epochManager:           epochManager,
		epochCommitmentFactory: epochCommitmentFactory,
		pendingConflictsCount:  make(map[epoch.EI]uint64),
		log:                    options.Log,
		options:                options,
		Events: &Events{
			EpochCommitted: event.New[*EpochCommittedEvent](),
		},
	}
}

func (m *Manager) LoadSnapshot(snapshot *ledger.Snapshot) error {
	snapshot.Outputs.ForEach(func(output utxo.Output) error {
		m.epochCommitmentFactory.storage.ledgerstateStore.Store(output).Release()
		return nil
	})
	m.epochCommitmentFactory.DiffEpochIndex = snapshot.DiffEpochIndex
	m.epochCommitmentFactory.FullEpochIndex = snapshot.FullEpochIndex
	for ei, diff := range snapshot.EpochDiffs {
		m.epochCommitmentFactory.storage.diffStores[ei].Store(diff).Release()
	}
	return nil
}

// PendingConflictsCount returns the current value of pendingConflictsCount.
func (m *Manager) PendingConflictsCount(ei epoch.EI) uint64 {
	m.pccMutex.RLock()
	defer m.pccMutex.RUnlock()
	return m.pendingConflictsCount[ei]
}

// IsCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) IsCommittable(ei epoch.EI) bool {
	t := m.epochManager.EIToStartTime(ei)
	diff := time.Since(t)
	return m.PendingConflictsCount(ei) == 0 && diff >= m.options.MinCommittableEpochAge
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func (m *Manager) GetLatestEC() *tangle.EpochCommitment {
	ei := m.epochManager.CurrentEI()
	for ei > 0 {
		if m.IsCommittable(ei) {
			break
		}
		ei -= 1
	}
	ec, err := m.epochCommitmentFactory.GetEpochCommitment(ei)
	if err != nil {
		m.log.Error(err)
	}
	return ec
}

// OnMessageConfirmed is the handler for message confirmed event.
func (m *Manager) OnMessageConfirmed(message *tangle.Message) {
	ei := m.epochManager.TimeToEI(message.IssuingTime())
	err := m.epochCommitmentFactory.InsertTangleLeaf(ei, message.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
}

// OnTransactionConfirmed is the handler for transaction confirmed event.
func (m *Manager) OnTransactionConfirmed(tx *devnetvm.Transaction) {
	earliestAttachment := m.tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
	ei := m.epochManager.TimeToEI(earliestAttachment.IssuingTime())
	err := m.epochCommitmentFactory.InsertStateMutationLeaf(ei, tx.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
	m.updateStateLeafs(ei, tx)
}

// OnTransactionInclusionUpdated is the handler for transaction inclusion updated event.
func (m *Manager) OnTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	prevEpoch := m.epochManager.TimeToEI(event.PreviousInclusionTime)
	newEpoch := m.epochManager.TimeToEI(event.InclusionTime)

	if prevEpoch == newEpoch {
		return
	}

	m.epochCommitmentFactory.RemoveStateMutationLeaf(prevEpoch, event.TransactionID)
	m.epochCommitmentFactory.InsertStateMutationLeaf(newEpoch, event.TransactionID)

	// TODO: propagate updates to future epochs
}

// OnBranchConfirmed is the handler for branch confirmed event.
func (m *Manager) OnBranchConfirmed(branchID utxo.TransactionID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei] -= 1
}

// OnBranchCreated is the handler for branch created event.
func (m *Manager) OnBranchCreated(branchID utxo.TransactionID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei] += 1
}

// OnBranchRejected is the handler for branch created event.
func (m *Manager) OnBranchRejected(branchID utxo.TransactionID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei] -= 1
}

func (m *Manager) updateStateLeafs(ei epoch.EI, tx *devnetvm.Transaction) {
	outputsSpent := m.tangle.Ledger.Utils.ResolveInputs(tx.Inputs())
	outputsCreated := tx.Essence().Outputs()

	// update the state root only if  ei is the next committable epoch
	if ei == m.epochCommitmentFactory.lastCommittedEpoch+1 {
		for _, o := range outputsCreated {
			err := m.epochCommitmentFactory.InsertStateLeaf(ei, o.ID())
			if err != nil && m.log != nil {
				m.log.Error(err)
			}
		}
		// remove spent outputs
		for it := outputsSpent.Iterator(); it.HasNext(); {
			err := m.epochCommitmentFactory.RemoveStateLeaf(ei, it.Next())
			if err != nil && m.log != nil {
				m.log.Error(err)
			}
		}
	}
	// store outputs in the commitment diff storage
	m.epochCommitmentFactory.storeDiffUTXOs(ei, outputsSpent, outputsCreated)
}

func (m *Manager) getBranchEI(branchID utxo.TransactionID) (ei epoch.EI) {
	m.tangle.Ledger.Storage.CachedTransaction(branchID).Consume(func(tx utxo.Transaction) {
		earliestAttachment := m.tangle.MessageFactory.EarliestAttachment(utxo.NewTransactionIDs(tx.ID()))
		ei = m.epochManager.TimeToEI(earliestAttachment.IssuingTime())
	})
	return
}

// GetBlockInclusionProof gets the proof of the inclusion (acceptance) of a block.
func (m *Manager) GetBlockInclusionProof(blockID tangle.MessageID) (*CommitmentProof, error) {
	var ei epoch.EI
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
	var ei epoch.EI
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
	EI epoch.EI
}
