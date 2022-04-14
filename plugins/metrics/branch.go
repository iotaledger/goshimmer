package metrics

import (
	"sync"

	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
)

var (
	// total number of branches in the database at startup.
	initialBranchTotalCountDB uint64

	// total number of finalized branches in the database at startup.
	initialFinalizedBranchCountDB uint64

	// total number of confirmed branches in the database at startup.
	initialConfirmedBranchCountDB uint64

	// number of branches created since the node started.
	branchTotalCountDB atomic.Uint64

	// number of branches finalized since the node started.
	finalizedBranchCountDB atomic.Uint64

	// current number of confirmed branches.
	confirmedBranchCount atomic.Uint64

	// total time it took all branches to finalize. unit is milliseconds!
	branchConfirmationTotalTime atomic.Uint64

	// all active branches stored in this map, to avoid duplicated event triggers for branch confirmation.
	activeBranches map[branchdag.BranchID]types.Empty

	activeBranchesMutex sync.Mutex
)

// BranchConfirmationTotalTime returns total time it took for all confirmed branches to be confirmed.
func BranchConfirmationTotalTime() uint64 {
	return branchConfirmationTotalTime.Load()
}

// ConfirmedBranchCount returns the number of confirmed branches.
func ConfirmedBranchCount() uint64 {
	return initialConfirmedBranchCountDB + confirmedBranchCount.Load()
}

// TotalBranchCountDB returns the total number of branches.
func TotalBranchCountDB() uint64 {
	return initialBranchTotalCountDB + branchTotalCountDB.Load()
}

// FinalizedBranchCountDB returns the number of non-confirmed branches.
func FinalizedBranchCountDB() uint64 {
	return initialFinalizedBranchCountDB + finalizedBranchCountDB.Load()
}

func measureInitialBranchStats() {
	activeBranchesMutex.Lock()
	defer activeBranchesMutex.Unlock()
	activeBranches = make(map[branchdag.BranchID]types.Empty)
	conflictsToRemove := make([]branchdag.BranchID, 0)
	deps.Tangle.Ledger.BranchDAG.Utils.ForEachBranch(func(branch *branchdag.Branch) {
		switch branch.ID() {
		case branchdag.MasterBranchID:
			return
		default:
			initialBranchTotalCountDB++
			activeBranches[branch.ID()] = types.Void
			branchGoF, err := deps.Tangle.Ledger.Utils.BranchGradeOfFinality(branch.ID())
			if err != nil {
				return
			}
			if branchGoF == gof.High {
				deps.Tangle.Ledger.BranchDAG.Utils.ForEachConflictingBranchID(branch.ID(), func(conflictingBranchID branchdag.BranchID) bool {
					if conflictingBranchID != branch.ID() {
						initialFinalizedBranchCountDB++
					}
					return true
				})
				initialFinalizedBranchCountDB++
				initialConfirmedBranchCountDB++
				conflictsToRemove = append(conflictsToRemove, branch.ID())
			}
		}
	})

	// remove finalized branches from the map in separate loop when all conflicting branches are known
	for _, branchID := range conflictsToRemove {
		deps.Tangle.Ledger.BranchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID branchdag.BranchID) bool {
			if conflictingBranchID != branchID {
				delete(activeBranches, conflictingBranchID)
			}
			return true
		})
		delete(activeBranches, branchID)
	}
}
