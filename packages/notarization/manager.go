package notarization

import (
	"github.com/iotaledger/hive.go/logger"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	minEpochCommitableDuration = 24 * time.Minute
)

// Manager is the notarization manager.
type Manager struct {
	tangle                 *tangle.Tangle
	epochManager           *EpochManager
	epochCommitmentFactory *EpochCommitmentFactory
	options                *ManagerOptions
	pendingConflictsCount  map[EI]uint64
	pccMutex               sync.RWMutex
	log                    *logger.Logger
}

// NewManager creates and returns a new notarization manager.
func NewManager(epochManager *EpochManager, epochCommitmentFactory *EpochCommitmentFactory, tangle *tangle.Tangle, opts ...ManagerOption) *Manager {
	options := &ManagerOptions{
		MinCommitableEpochAge: minEpochCommitableDuration,
		Log:                   nil,
	}
	for _, option := range opts {
		option(options)
	}
	return &Manager{
		tangle:                 tangle,
		epochManager:           epochManager,
		epochCommitmentFactory: epochCommitmentFactory,
		pendingConflictsCount:  make(map[EI]uint64),
		log:                    options.Log,
		options:                options,
	}
}

// PendingConflictsCount returns the current value of pendingConflictsCount.
func (m *Manager) PendingConflictsCount(ei EI) uint64 {
	m.pccMutex.RLock()
	defer m.pccMutex.RUnlock()
	return m.pendingConflictsCount[ei]
}

// IsCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) IsCommittable(ei EI) bool {
	t := m.epochManager.EIToStartTime(ei)
	diff := time.Since(t)
	return m.PendingConflictsCount(ei) == 0 && diff >= m.options.MinCommitableEpochAge
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func (m *Manager) GetLatestEC() *EpochCommitment {
	ei := m.epochManager.CurrentEI()
	for ei >= 0 {
		if m.IsCommittable(ei) {
			break
		}
		ei -= 1
	}
	return m.epochCommitmentFactory.GetEpochCommitment(ei)
}

// GetBlockInclusionProof gets the proof of the inclusion (acceptance) of a block.
func (m *Manager) GetBlockInclusionProof(blockID tangle.MessageID) (*CommitmentProof, error) {
	var ei EI
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
func (m *Manager) GetTransactionInclusionProof(transactionID ledgerstate.TransactionID) (*CommitmentProof, error) {
	var ei EI
	m.tangle.LedgerState.Transaction(transactionID).Consume(func(tx *ledgerstate.Transaction) {
		t := tx.Essence().Timestamp()
		ei = m.epochManager.TimeToEI(t)
	})
	proof, err := m.epochCommitmentFactory.ProofStateMutationRoot(ei, transactionID)
	if err != nil {
		return nil, err
	}
	return proof, nil
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
func (m *Manager) OnTransactionConfirmed(tx *ledgerstate.Transaction) {
	ei := m.epochManager.TimeToEI(tx.Essence().Timestamp())
	err := m.epochCommitmentFactory.InsertStateMutationLeaf(ei, tx.ID())
	if err != nil && m.log != nil {
		m.log.Error(err)
	}
	m.updateStateSMT(ei, tx)
}

func (m *Manager) updateStateSMT(ei EI, tx *ledgerstate.Transaction) {
	for _, o := range tx.Essence().Outputs() {
		err := m.epochCommitmentFactory.InsertStateLeaf(ei, o.ID())
		if err != nil && m.log != nil {
			m.log.Error(err)
		}
	}
	// remove spent outputs
	for _, i := range tx.Essence().Inputs() {
		out, _ := ledgerstate.OutputIDFromBase58(i.Base58())
		err := m.epochCommitmentFactory.RemoveStateLeaf(ei, out)
		if err != nil && m.log != nil {
			m.log.Error(err)
		}
	}
}

// OnBranchConfirmed is the handler for branch confirmed event.
func (m *Manager) OnBranchConfirmed(branchID ledgerstate.BranchID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei] -= 1
}

// OnBranchCreated is the handler for branch created event.
func (m *Manager) OnBranchCreated(branchID ledgerstate.BranchID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei] += 1
}

// OnBranchRejected is the handler for branch created event.
func (m *Manager) OnBranchRejected(branchID ledgerstate.BranchID) {
	m.pccMutex.Lock()
	defer m.pccMutex.Unlock()

	ei := m.getBranchEI(branchID)
	m.pendingConflictsCount[ei] -= 1
}

func (m *Manager) getBranchEI(branchID ledgerstate.BranchID) EI {
	tx := m.tangle.LedgerState.Ledgerstate.Transaction(branchID.TransactionID())
	ei := m.epochManager.TimeToEI(tx.Essence().Timestamp())
	return ei
}

// ManagerOption represents the return type of the optional config parameters of the notarization manager.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container of all the config parameters of the notarization manager.
type ManagerOptions struct {
	MinCommitableEpochAge time.Duration
	Log                   *logger.Logger
}

// MinCommitableEpochAge specifies how old an epoch has to be for it to be commitable.
func MinCommitableEpochAge(d time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.MinCommitableEpochAge = d
	}
}

// Log provides the logger.
func Log(log *logger.Logger) ManagerOption {
	return func(options *ManagerOptions) {
		options.Log = log
	}
}
