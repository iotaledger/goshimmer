package notarization

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Manager struct {
	// TODO: epochManager

	// pending branch counter
	pendingBranchesCount map[ECI]uint64
	pbcMutex             sync.RWMutex
}

// NewManager creates and returns a new notarization manager.
func NewManager() *Manager {
	return &Manager{
		pendingBranchesCount: make(map[ECI]uint64),
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
	return true
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func (m *Manager) GetLatestEC() *EpochCommitment {
	return nil
}

func (m *Manager) OnMessageConfirmed(messageID tangle.MessageID) {

}

func (m *Manager) OnTransactionConfirmed(txID ledgerstate.TransactionID) {

}

func (m *Manager) OnBranchConfirmed(branchID ledgerstate.BranchID) {

}

func (m *Manager) OnBranchCreated(branchID ledgerstate.BranchID) {

}
