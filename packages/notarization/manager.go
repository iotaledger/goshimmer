package notarization

import (
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
	epochManager           *EpochManager
	epochCommitmentFactory *EpochCommitmentFactory
	options                *ManagerOptions
	pendingBranchesCount   map[ECI]uint64
	pbcMutex               sync.RWMutex
}

// NewManager creates and returns a new notarization manager.
func NewManager(epochManager *EpochManager, epochCommitmentFactory *EpochCommitmentFactory, opts ...ManagerOption) *Manager {
	options := &ManagerOptions{
		MinCommitableEpochAge: minEpochCommitableDuration,
	}
	for _, option := range opts {
		option(options)
	}
	return &Manager{
		epochManager:           epochManager,
		epochCommitmentFactory: epochCommitmentFactory,
		pendingBranchesCount:   make(map[ECI]uint64),
		options:                options,
	}
}

// PendingBranchesCount returns the current value of pendingBranchesCount.
func (m *Manager) PendingBranchesCount(eci ECI) uint64 {
	m.pbcMutex.RLock()
	defer m.pbcMutex.RUnlock()
	return m.pendingBranchesCount[eci]
}

// IsCommittable returns if the epoch is committable, if all conflicts are resolved and the epoch is old enough.
func (m *Manager) IsCommittable(eci ECI) bool {
	t := m.epochManager.ECIToStartTime(eci)
	diff := time.Since(t)
	return m.PendingBranchesCount(eci) == 0 && diff >= m.options.MinCommitableEpochAge
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func (m *Manager) GetLatestEC() *EpochCommitment {
	eci := m.epochManager.CurrentECI()
	for eci >= 0 {
		if m.IsCommittable(eci) {
			break
		}
		eci -= 1
	}
	return m.epochCommitmentFactory.GetCommitment(eci)
}

// OnMessageConfirmed is the handler for message confirmed event.
func (m *Manager) OnMessageConfirmed(message *tangle.Message) {
	eci := m.epochManager.TimeToECI(message.IssuingTime())
	m.epochCommitmentFactory.InsertTangleLeaf(eci, message.ID())
}

// OnTransactionConfirmed is the handler for transaction confirmed event.
func (m *Manager) OnTransactionConfirmed(tx *ledgerstate.Transaction) {
	eci := m.epochManager.TimeToECI(tx.Essence().Timestamp())
	m.epochCommitmentFactory.InsertStateMutationLeaf(eci, tx.ID())
	m.updateLedgerstateSMT(eci, tx)
}

func (m *Manager) updateLedgerstateSMT(eci ECI, tx *ledgerstate.Transaction) {
	for _, o := range tx.Essence().Outputs() {
		m.epochCommitmentFactory.InsertStateLeaf(eci, o.ID())
	}
	// remove spent outputs
	for _, i := range tx.Essence().Inputs() {
		out, _ := ledgerstate.OutputIDFromBase58(i.Base58())
		m.epochCommitmentFactory.RemoveStateLeaf(eci, out)
	}
}

// OnBranchConfirmed is the handler for branch confirmed event.
func (m *Manager) OnBranchConfirmed(branchID ledgerstate.BranchID) {

}

// OnBranchCreated is the handler for branch created event.
func (m *Manager) OnBranchCreated(branchID ledgerstate.BranchID) {

}

// ManagerOption represents the return type of the optional config parameters of the notarization manager.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container of all the config parameters of the notarization manager.
type ManagerOptions struct {
	MinCommitableEpochAge time.Duration
}

// MinCommitableEpochAge specifies how old an epoch has to be for it to be commitable.
func MinCommitableEpochAge(d time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.MinCommitableEpochAge = d
	}
}
