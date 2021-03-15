package tangle

import (
	fmt "fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApprovalWeightManager_updateBranchSupporters(t *testing.T) {
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
		approvalWeightManager.updateBranchSupporters(message)

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
		approvalWeightManager.updateBranchSupporters(message)

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
		approvalWeightManager.updateBranchSupporters(message)

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

func TestApprovalWeightManager_updateSequenceSupporters(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()
	_ = NewApprovalWeightManager(tangle)

	markersMap := make(map[string]*markers.StructureDetails)
	markersMap["1,1"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails(nil, increaseIndexCallback, markers.NewSequenceAlias([]byte("1")))
	markersMap["1,2"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,1"]}, increaseIndexCallback)
	markersMap["1,3"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,2"]}, increaseIndexCallback)
	markersMap["1,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,3"]}, increaseIndexCallback)

	markersMap["2,1"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails(nil, increaseIndexCallback, markers.NewSequenceAlias([]byte("2")))
	markersMap["2,2"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,1"]}, increaseIndexCallback)
	markersMap["2,3"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,2"]}, increaseIndexCallback)
	markersMap["2,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,3"]}, increaseIndexCallback)

	markersMap["3,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,3"], markersMap["2,3"]}, increaseIndexCallback)
	markersMap["3,5"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,4"], markersMap["3,4"]}, increaseIndexCallback)
	markersMap["3,6"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["3,5"]}, increaseIndexCallback)
	markersMap["3,7"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["3,6"]}, increaseIndexCallback)

	markersMap["4,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["3,7"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("4")))
	markersMap["5,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,4"], markersMap["3,7"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("5")))

	fmt.Println(markersMap["4,8"])
	fmt.Println(markersMap["5,8"])

	tangle.Booker.MarkersManager.Manager.Sequence(3).Consume(func(sequence *markers.Sequence) {
		sequence.ParentReferences().HighestReferencedMarkers(7).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			fmt.Println(sequenceID, index)

			return true
		})
	})
}

func increaseIndexCallback(sequenceID markers.SequenceID, currentHighestIndex markers.Index) bool {
	return true
}

func createBranch(t *testing.T, tangle *Tangle, branchID ledgerstate.BranchID, parentBranchID ledgerstate.BranchID, conflictID ledgerstate.ConflictID) {
	cachedBranch, _, err := tangle.LedgerState.branchDAG.CreateConflictBranch(branchID, ledgerstate.NewBranchIDs(parentBranchID), ledgerstate.NewConflictIDs(conflictID))
	require.NoError(t, err)

	cachedBranch.Release()
}

func validateStatementResults(t *testing.T, approvalWeightManager *ApprovalWeightManager, branchIDs map[string]ledgerstate.BranchID, issuerPublicKey ed25519.PublicKey, expectedResults map[string]bool) {
	for branchIDstring, expectedResult := range expectedResults {
		var actualResult bool
		supporters := approvalWeightManager.branchSupporters[branchIDs[branchIDstring]]
		if supporters != nil {
			actualResult = supporters.Has(issuerPublicKey)
		}

		assert.Equalf(t, expectedResult, actualResult, "%s(%s) does not match", branchIDstring, branchIDs[branchIDstring])
	}
}
