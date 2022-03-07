package tangle

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/events"
)

type PendingOpinionCache struct {
	likedBranchIDs    ledgerstate.BranchIDs
	dislikedBranchIDs ledgerstate.BranchIDs

	tangle *Tangle
}

func NewPendingOpinionCache(tangle *Tangle) *PendingOpinionCache {
	return &PendingOpinionCache{
		likedBranchIDs:    ledgerstate.NewBranchIDs(),
		dislikedBranchIDs: ledgerstate.NewBranchIDs(),
	}
}

func (p *PendingOpinionCache) Setup() {
	p.tangle.ApprovalWeightManager.Events.BranchWeightChanged.Attach(events.NewClosure(p.onBranchWeightChanged))
	p.tangle.ConfirmationOracle.Events().BranchConfirmed.Attach(events.NewClosure(p.onBranchConfirmed))
}

func (p *PendingOpinionCache) onBranchWeightChanged(branchID ledgerstate.BranchID, weight float64) {
	if _, exists := p.likedBranchIDs[branchID]; exists {

	}
}

func (p *PendingOpinionCache) onBranchConfirmed(branchID ledgerstate.BranchID) {
	p.tangle.LedgerState.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) bool {
		p.deletePendingBranch(conflictingBranchID)
		return true
	})

	p.deletePendingBranch(branchID)
}

func (p *PendingOpinionCache) deletePendingBranch(branchID ledgerstate.BranchID) {
	p.likedBranchIDs.Delete(branchID)
	p.dislikedBranchIDs.Delete(branchID)
}
