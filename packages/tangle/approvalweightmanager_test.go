package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/stretchr/testify/require"
)

func TestApprovalWeightManager_ProcessMessage(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()

	conflictIDs := map[string]ledgerstate.ConflictID{
		"Conflict 1": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 2": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 3": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 4": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 5": ledgerstate.ConflictIDFromRandomness(),
	}

	branchIDs := map[string]ledgerstate.BranchID{
		"Branch 1": ledgerstate.BranchIDFromRandomness(),
		"Branch 1.1": ledgerstate.BranchIDFromRandomness(),
		"Branch 1.2": ledgerstate.BranchIDFromRandomness(),
		"Branch 1.3": ledgerstate.BranchIDFromRandomness(),
		"Branch 2": ledgerstate.BranchIDFromRandomness(),
		"Branch 3": ledgerstate.BranchIDFromRandomness(),
		"Branch 4": ledgerstate.BranchIDFromRandomness(),
		"Branch 4.1": ledgerstate.BranchIDFromRandomness(),
		"Branch 4.1.1": ledgerstate.BranchIDFromRandomness(),
		"Branch 4.1.2": ledgerstate.BranchIDFromRandomness(),
		"Branch 4.2": ledgerstate.BranchIDFromRandomness(),
	}

	createBranch(t, tangle, branchIDs["Branch 1"], ledgerstate.MasterBranchID, conflictIDs["Conflict 1"])
	createBranch(t, tangle, branchIDs["Branch 2"], ledgerstate.MasterBranchID, conflictIDs["Conflict 1"])
	createBranch(t, tangle, branchIDs["Branch 3"], ledgerstate.MasterBranchID, conflictIDs["Conflict 2"])
	createBranch(t, tangle, branchIDs["Branch 4"], ledgerstate.MasterBranchID, conflictIDs["Conflict 2"])

	approvalWeightManager := NewApprovalWeightManager(tangle)
	approvalWeightManager.ProcessMessage(...)
}

func createBranch(t *testing.T, tangle *Tangle, branchID ledgerstate.BranchID, parentBranchID ledgerstate.BranchID, conflictID ledgerstate.ConflictID) {
	cachedBranch, _, err := tangle.LedgerState.branchDAG.CreateConflictBranch(branchID, ledgerstate.NewBranchIDs(parentBranchID), ledgerstate.NewConflictIDs(conflictID))
	require.NoError(t, err)

	cachedBranch.Release()
}