package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApprovalWeightManager_ProcessMessage(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()
	approvalWeightManager := NewApprovalWeightManager(tangle)

	keyPair := ed25519.GenerateKeyPair()

	conflictIDs := map[string]ledgerstate.ConflictID{
		"Conflict 1": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 2": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 3": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 4": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 5": ledgerstate.ConflictIDFromRandomness(),
	}

	branchIDs := map[string]ledgerstate.BranchID{
		"Branch 1":     ledgerstate.BranchIDFromRandomness(),
		"Branch 1.1":   ledgerstate.BranchIDFromRandomness(),
		"Branch 1.2":   ledgerstate.BranchIDFromRandomness(),
		"Branch 1.3":   ledgerstate.BranchIDFromRandomness(),
		"Branch 2":     ledgerstate.BranchIDFromRandomness(),
		"Branch 3":     ledgerstate.BranchIDFromRandomness(),
		"Branch 4":     ledgerstate.BranchIDFromRandomness(),
		"Branch 4.1":   ledgerstate.BranchIDFromRandomness(),
		"Branch 4.1.1": ledgerstate.BranchIDFromRandomness(),
		"Branch 4.1.2": ledgerstate.BranchIDFromRandomness(),
		"Branch 4.2":   ledgerstate.BranchIDFromRandomness(),
	}

	createBranch(t, tangle, branchIDs["Branch 1"], ledgerstate.MasterBranchID, conflictIDs["Conflict 1"])
	createBranch(t, tangle, branchIDs["Branch 2"], ledgerstate.MasterBranchID, conflictIDs["Conflict 1"])
	createBranch(t, tangle, branchIDs["Branch 3"], ledgerstate.MasterBranchID, conflictIDs["Conflict 2"])
	createBranch(t, tangle, branchIDs["Branch 4"], ledgerstate.MasterBranchID, conflictIDs["Conflict 2"])

	createBranch(t, tangle, branchIDs["Branch 1.1"], branchIDs["Branch 1"], conflictIDs["Conflict 3"])
	createBranch(t, tangle, branchIDs["Branch 1.2"], branchIDs["Branch 1"], conflictIDs["Conflict 3"])
	createBranch(t, tangle, branchIDs["Branch 1.3"], branchIDs["Branch 1"], conflictIDs["Conflict 3"])

	createBranch(t, tangle, branchIDs["Branch 4.1"], branchIDs["Branch 4"], conflictIDs["Conflict 4"])
	createBranch(t, tangle, branchIDs["Branch 4.2"], branchIDs["Branch 4"], conflictIDs["Conflict 4"])

	createBranch(t, tangle, branchIDs["Branch 4.1.1"], branchIDs["Branch 4.1"], conflictIDs["Conflict 5"])
	createBranch(t, tangle, branchIDs["Branch 4.1.2"], branchIDs["Branch 4.1"], conflictIDs["Conflict 5"])

	cachedAggregatedBranch, _, err := tangle.LedgerState.branchDAG.AggregateBranches(ledgerstate.NewBranchIDs(branchIDs["Branch 1.1"], branchIDs["Branch 4.1.1"]))
	require.NoError(t, err)
	cachedAggregatedBranch.Consume(func(branch ledgerstate.Branch) {
		branchIDs["Branch 1.1 + Branch 4.1.1"] = branch.ID()
	})

	// statement 1: "Branch 1.1 + Branch 4.1.1"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 1.1 + Branch 4.1.1"])
		})
		approvalWeightManager.ProcessMessage(message.ID())

		expectedResults := map[string]bool{
			"Branch 1":     true,
			"Branch 1.1":   true,
			"Branch 1.2":   false,
			"Branch 1.3":   false,
			"Branch 2":     false,
			"Branch 3":     false,
			"Branch 4":     true,
			"Branch 4.1":   true,
			"Branch 4.1.1": true,
			"Branch 4.1.2": false,
			"Branch 4.2":   false,
		}
		validateStatementResults(t, approvalWeightManager, branchIDs, keyPair.PublicKey, expectedResults)
	}

	// statement 2: "Branch 4.1.2"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 4.1.2"])
		})
		approvalWeightManager.ProcessMessage(message.ID())

		expectedResults := map[string]bool{
			"Branch 1":     true,
			"Branch 1.1":   true,
			"Branch 1.2":   false,
			"Branch 1.3":   false,
			"Branch 2":     false,
			"Branch 3":     false,
			"Branch 4":     true,
			"Branch 4.1":   true,
			"Branch 4.1.1": false,
			"Branch 4.1.2": true,
			"Branch 4.2":   false,
		}
		validateStatementResults(t, approvalWeightManager, branchIDs, keyPair.PublicKey, expectedResults)
	}

	// statement 3: "Branch 2"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 2"])
		})
		approvalWeightManager.ProcessMessage(message.ID())

		expectedResults := map[string]bool{
			"Branch 1":     false,
			"Branch 1.1":   false,
			"Branch 1.2":   false,
			"Branch 1.3":   false,
			"Branch 2":     true,
			"Branch 3":     false,
			"Branch 4":     true,
			"Branch 4.1":   true,
			"Branch 4.1.1": false,
			"Branch 4.1.2": true,
			"Branch 4.2":   false,
		}
		validateStatementResults(t, approvalWeightManager, branchIDs, keyPair.PublicKey, expectedResults)
	}
}

func createBranch(t *testing.T, tangle *Tangle, branchID ledgerstate.BranchID, parentBranchID ledgerstate.BranchID, conflictID ledgerstate.ConflictID) {
	cachedBranch, _, err := tangle.LedgerState.branchDAG.CreateConflictBranch(branchID, ledgerstate.NewBranchIDs(parentBranchID), ledgerstate.NewConflictIDs(conflictID))
	require.NoError(t, err)

	cachedBranch.Release()
}

func validateStatementResults(t *testing.T, approvalWeightManager *ApprovalWeightManager, branchIDs map[string]ledgerstate.BranchID, issuerPublicKey ed25519.PublicKey, expectedResults map[string]bool) {
	for branchIDstring, expectedResult := range expectedResults {
		var actualResult bool
		supporters := approvalWeightManager.supporters[branchIDs[branchIDstring]]
		if supporters != nil {
			actualResult = supporters.Has(issuerPublicKey)
		}

		assert.Equalf(t, expectedResult, actualResult, "%s(%s) does not match", branchIDstring, branchIDs[branchIDstring])
	}
}
