package tangle

import (
	"fmt"
	"math"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestBranchWeightMarshalling(t *testing.T) {
	branchWeight := NewBranchWeight(ledgerstate.BranchIDFromRandomness())
	branchWeight.SetWeight(5.1234)

	branchWeightFromBytes, _, err := BranchWeightFromBytes(branchWeight.Bytes())
	require.NoError(t, err)

	assert.Equal(t, branchWeight.Bytes(), branchWeightFromBytes.Bytes())
	assert.Equal(t, branchWeight.BranchID(), branchWeightFromBytes.BranchID())
	assert.Equal(t, branchWeight.Weight(), branchWeightFromBytes.Weight())
}

func TestStatementMarshalling(t *testing.T) {
	statement := NewStatement(ledgerstate.BranchIDFromRandomness(), identity.GenerateIdentity().ID())
	statement.UpdateSequenceNumber(10)
	statementFromBytes, _, err := StatementFromBytes(statement.Bytes())
	require.NoError(t, err)

	assert.Equal(t, statement.Bytes(), statementFromBytes.Bytes())
	assert.Equal(t, statement.BranchID(), statementFromBytes.BranchID())
	assert.Equal(t, statement.Supporter(), statementFromBytes.Supporter())
	assert.Equal(t, statement.SequenceNumber(), statementFromBytes.SequenceNumber())
}

func TestBranchSupportersMarshalling(t *testing.T) {
	branchSupporters := NewBranchSupporters(ledgerstate.BranchIDFromRandomness())

	for i := 0; i < 100; i++ {
		branchSupporters.AddSupporter(identity.GenerateIdentity().ID())
	}

	branchSupportersFromBytes, _, err := BranchSupportersFromBytes(branchSupporters.Bytes())
	require.NoError(t, err)

	// verify that branchSupportersFromBytes has all supporters from branchSupporters
	assert.Equal(t, branchSupporters.Supporters().Size(), branchSupportersFromBytes.Supporters().Size())
	branchSupporters.Supporters().ForEach(func(supporter Supporter) {
		assert.True(t, branchSupportersFromBytes.supporters.Has(supporter))
	})
}

// TestApprovalWeightManager_updateBranchSupporters tests the ApprovalWeightManager's functionality regarding branches.
// The scenario can be found in images/approvalweight-updateBranchSupporters.png.
func TestApprovalWeightManager_updateBranchSupporters(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	manaRetrieverMock := func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			identity.NewID(keyPair.PublicKey): 100,
		}
	}
	manager := epochs.NewManager(epochs.ManaRetriever(manaRetrieverMock), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)))
	defer tangle.Shutdown()
	approvalWeightManager := tangle.ApprovalWeightManager

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

	createBranch(t, tangle, "Branch 1", branchIDs, ledgerstate.MasterBranchID, conflictIDs["Conflict 1"])
	createBranch(t, tangle, "Branch 2", branchIDs, ledgerstate.MasterBranchID, conflictIDs["Conflict 1"])
	createBranch(t, tangle, "Branch 3", branchIDs, ledgerstate.MasterBranchID, conflictIDs["Conflict 2"])
	createBranch(t, tangle, "Branch 4", branchIDs, ledgerstate.MasterBranchID, conflictIDs["Conflict 2"])

	createBranch(t, tangle, "Branch 1.1", branchIDs, branchIDs["Branch 1"], conflictIDs["Conflict 3"])
	createBranch(t, tangle, "Branch 1.2", branchIDs, branchIDs["Branch 1"], conflictIDs["Conflict 3"])
	createBranch(t, tangle, "Branch 1.3", branchIDs, branchIDs["Branch 1"], conflictIDs["Conflict 3"])

	createBranch(t, tangle, "Branch 4.1", branchIDs, branchIDs["Branch 4"], conflictIDs["Conflict 4"])
	createBranch(t, tangle, "Branch 4.2", branchIDs, branchIDs["Branch 4"], conflictIDs["Conflict 4"])

	createBranch(t, tangle, "Branch 4.1.1", branchIDs, branchIDs["Branch 4.1"], conflictIDs["Conflict 5"])
	createBranch(t, tangle, "Branch 4.1.2", branchIDs, branchIDs["Branch 4.1"], conflictIDs["Conflict 5"])

	cachedAggregatedBranch, _, err := tangle.LedgerState.BranchDAG.AggregateBranches(ledgerstate.NewBranchIDs(branchIDs["Branch 1.1"], branchIDs["Branch 4.1.1"]))
	require.NoError(t, err)
	cachedAggregatedBranch.Consume(func(branch ledgerstate.Branch) {
		branchIDs["Branch 1.1 + Branch 4.1.1"] = branch.ID()
		ledgerstate.RegisterBranchIDAlias(branch.ID(), "Branch 1.1 + Branch 4.1.1")
	})

	// Issue statements in different order to make sure that no information is lost when nodes apply statements in arbitrary order

	message1 := newTestDataMessagePublicKey("test1", keyPair.PublicKey)
	message2 := newTestDataMessagePublicKey("test2", keyPair.PublicKey)
	// statement 2: "Branch 4.1.2"
	{
		message := message2
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement2")
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 4.1.2"])
		})
		approvalWeightManager.updateBranchSupporters(message)

		expectedResults := map[string]bool{
			"Branch 1":     false,
			"Branch 1.1":   false,
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
		validateStatementResults(t, approvalWeightManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}

	// statement 1: "Branch 1.1 + Branch 4.1.1"
	{
		message := message1
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement1")
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
			"Branch 4.1.1": false,
			"Branch 4.1.2": true,
			"Branch 4.2":   false,
		}
		validateStatementResults(t, approvalWeightManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}

	//// statement 3: "Branch 2"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement3")
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
		validateStatementResults(t, approvalWeightManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}
}

// TestApprovalWeightManager_updateSequenceSupporters tests the ApprovalWeightManager's functionality regarding sequences.
// The scenario can be found in images/approvalweight-updateSequenceSupporters.png.
func TestApprovalWeightManager_updateSequenceSupporters(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	manaRetrieverMock := func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			identity.NewID(keyPair.PublicKey): 100,
		}
	}
	manager := epochs.NewManager(epochs.ManaRetriever(manaRetrieverMock), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)))
	defer tangle.Shutdown()
	approvalWeightManager := tangle.ApprovalWeightManager
	supporters := map[string]*identity.Identity{
		"A": identity.New(ed25519.GenerateKeyPair().PublicKey),
		"B": identity.New(ed25519.GenerateKeyPair().PublicKey),
	}
	markersMap := make(map[string]*markers.StructureDetails)

	// build markers DAG
	{
		markersMap["1,1"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails(nil, increaseIndexCallback, markers.NewSequenceAlias([]byte("1")))
		markersMap["1,2"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,1"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("1")))
		markersMap["1,3"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,2"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("1")))
		markersMap["1,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,3"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("1")))
		markersMap["2,1"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails(nil, increaseIndexCallback, markers.NewSequenceAlias([]byte("2")))
		markersMap["2,2"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,1"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("2")))
		markersMap["2,3"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,2"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("2")))
		markersMap["2,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,3"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("2")))
		markersMap["3,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,3"], markersMap["2,3"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("3")))
		markersMap["3,5"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,4"], markersMap["3,4"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("3")))
		markersMap["3,6"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["3,5"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("3")))
		markersMap["3,7"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["3,6"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("3")))
		markersMap["4,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["3,7"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("4")))
		markersMap["5,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["3,7"], markersMap["2,4"]}, increaseIndexCallback, markers.NewSequenceAlias([]byte("5")))
	}

	// CASE1: APPROVE MARKER(1, 3)
	{
		approvalWeightManager.updateSequenceSupporters(approveMarkers(approvalWeightManager, supporters["A"], markers.NewMarker(1, 3)))

		validateMarkerSupporters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"1,1": {supporters["A"]},
			"1,2": {supporters["A"]},
			"1,3": {supporters["A"]},
			"1,4": {},
			"2,1": {},
			"2,2": {},
			"2,3": {},
			"2,4": {},
			"3,4": {},
			"3,5": {},
			"3,6": {},
			"3,7": {},
			"4,8": {},
			"5,8": {},
		})
	}

	// CASE2: APPROVE MARKER(1, 4) + MARKER(3, 5)
	{
		approvalWeightManager.updateSequenceSupporters(approveMarkers(approvalWeightManager, supporters["A"], markers.NewMarker(1, 4), markers.NewMarker(3, 5)))

		validateMarkerSupporters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"1,1": {supporters["A"]},
			"1,2": {supporters["A"]},
			"1,3": {supporters["A"]},
			"1,4": {supporters["A"]},
			"2,1": {supporters["A"]},
			"2,2": {supporters["A"]},
			"2,3": {supporters["A"]},
			"2,4": {},
			"3,4": {supporters["A"]},
			"3,5": {supporters["A"]},
			"3,6": {},
			"3,7": {},
			"4,8": {},
			"5,8": {},
		})
	}

	// CASE3: APPROVE MARKER(5, 8)
	{
		approvalWeightManager.updateSequenceSupporters(approveMarkers(approvalWeightManager, supporters["A"], markers.NewMarker(5, 8)))

		validateMarkerSupporters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"1,1": {supporters["A"]},
			"1,2": {supporters["A"]},
			"1,3": {supporters["A"]},
			"1,4": {supporters["A"]},
			"2,1": {supporters["A"]},
			"2,2": {supporters["A"]},
			"2,3": {supporters["A"]},
			"2,4": {supporters["A"]},
			"3,4": {supporters["A"]},
			"3,5": {supporters["A"]},
			"3,6": {supporters["A"]},
			"3,7": {supporters["A"]},
			"4,8": {},
			"5,8": {supporters["A"]},
		})
	}

	// CASE4: APPROVE MARKER(2, 3)
	{
		approvalWeightManager.updateSequenceSupporters(approveMarkers(approvalWeightManager, supporters["B"], markers.NewMarker(2, 3)))

		validateMarkerSupporters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"1,1": {supporters["A"]},
			"1,2": {supporters["A"]},
			"1,3": {supporters["A"]},
			"1,4": {supporters["A"]},
			"2,1": {supporters["A"], supporters["B"]},
			"2,2": {supporters["A"], supporters["B"]},
			"2,3": {supporters["A"], supporters["B"]},
			"2,4": {supporters["A"]},
			"3,4": {supporters["A"]},
			"3,5": {supporters["A"]},
			"3,6": {supporters["A"]},
			"3,7": {supporters["A"]},
			"4,8": {},
			"5,8": {supporters["A"]},
		})
	}
}

// TestApprovalWeightManager_ProcessMessage tests the whole functionality of the ApprovalWeightManager.
// The scenario can be found in images/approvalweight-processMessage.png.
func TestApprovalWeightManager_ProcessMessage(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	manager := epochs.NewManager(epochs.ManaRetriever(func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)))
	defer tangle.Shutdown()
	tangle.Setup()

	testEventMock := newEventMock(t, tangle.ApprovalWeightManager)
	testFramework := NewMessageTestFramework(tangle, WithGenesisOutput("A", 500))

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

		issueAndValidateMessageApproval(t, "Message1", testEventMock, testFramework, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.30,
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuer(nodes["B"].PublicKey()))

		issueAndValidateMessageApproval(t, "Message2", testEventMock, testFramework, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.45,
			*markers.NewMarker(1, 2): 0.15,
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["C"].PublicKey()))

		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(1, 1), 1, events.ThresholdLevelIncreased)

		issueAndValidateMessageApproval(t, "Message3", testEventMock, testFramework, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.70,
			*markers.NewMarker(1, 2): 0.40,
			*markers.NewMarker(1, 3): 0.25,
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()))

		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(1, 2), 1, events.ThresholdLevelIncreased)

		issueAndValidateMessageApproval(t, "Message4", testEventMock, testFramework, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.90,
			*markers.NewMarker(1, 2): 0.60,
			*markers.NewMarker(1, 3): 0.45,
			*markers.NewMarker(1, 4): 0.20,
		})
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("B", 500))
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message5")), "Branch1")

		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(1, 3), 1, events.ThresholdLevelIncreased)
		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(1, 4), 1, events.ThresholdLevelIncreased)

		issueAndValidateMessageApproval(t, "Message5", testEventMock, testFramework, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.90,
			*markers.NewMarker(1, 2): 0.90,
			*markers.NewMarker(1, 3): 0.75,
			*markers.NewMarker(1, 4): 0.50,
			*markers.NewMarker(1, 5): 0.30,
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message4"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A"), WithOutput("C", 500))
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message6")), "Branch2")

		issueAndValidateMessageApproval(t, "Message6", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"): 0.3,
			testFramework.BranchID("Message6"): 0.1,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 0.85,
			*markers.NewMarker(1, 4): 0.60,
			*markers.NewMarker(1, 5): 0.30,
			*markers.NewMarker(2, 5): 0.10,
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message5"), WithIssuer(nodes["C"].PublicKey()), WithInputs("B"), WithOutput("E", 500))

		testFramework.PreventNewMarkers(true)
		issueAndValidateMessageApproval(t, "Message7", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"): 0.55,
			testFramework.BranchID("Message6"): 0.1,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 0.85,
			*markers.NewMarker(1, 4): 0.85,
			*markers.NewMarker(1, 5): 0.55,
			*markers.NewMarker(2, 5): 0.10,
		})
		testFramework.PreventNewMarkers(false)
	}

	// ISSUE Message7.1
	{
		testFramework.CreateMessage("Message7.1", WithStrongParents("Message7"), WithIssuer(nodes["A"].PublicKey()))

		testFramework.PreventNewMarkers(true)
		issueAndValidateMessageApproval(t, "Message7.1", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"): 0.55,
			testFramework.BranchID("Message6"): 0.1,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 0.85,
			*markers.NewMarker(1, 4): 0.85,
			*markers.NewMarker(1, 5): 0.55,
			*markers.NewMarker(2, 5): 0.10,
		})
		testFramework.PreventNewMarkers(false)
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message6"), WithIssuer(nodes["D"].PublicKey()))

		issueAndValidateMessageApproval(t, "Message8", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"): 0.55,
			testFramework.BranchID("Message6"): 0.30,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 0.85,
			*markers.NewMarker(1, 4): 0.85,
			*markers.NewMarker(1, 5): 0.55,
			*markers.NewMarker(2, 5): 0.30,
			*markers.NewMarker(2, 6): 0.20,
		})
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message8"), WithIssuer(nodes["A"].PublicKey()))

		issueAndValidateMessageApproval(t, "Message9", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"): 0.25,
			testFramework.BranchID("Message6"): 0.60,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 0.85,
			*markers.NewMarker(1, 4): 0.85,
			*markers.NewMarker(1, 5): 0.25,
			*markers.NewMarker(2, 5): 0.60,
			*markers.NewMarker(2, 6): 0.50,
			*markers.NewMarker(2, 7): 0.30,
		})
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("BranchConfirmed", testFramework.BranchID("Message6"), 1, events.ThresholdLevelIncreased)
		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(2, 5), 1, events.ThresholdLevelIncreased)
		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(2, 6), 1, events.ThresholdLevelIncreased)

		issueAndValidateMessageApproval(t, "Message10", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"): 0.25,
			testFramework.BranchID("Message6"): 0.75,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 1,
			*markers.NewMarker(1, 5): 0.25,
			*markers.NewMarker(2, 5): 0.75,
			*markers.NewMarker(2, 6): 0.65,
			*markers.NewMarker(2, 7): 0.45,
			*markers.NewMarker(2, 8): 0.15,
		})
	}

	// ISSUE Message11
	{
		testFramework.CreateMessage("Message11", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message7")), "Branch3")
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message11")), "Branch4")

		testEventMock.Expect("BranchConfirmed", testFramework.BranchID("Message7"), 1, events.ThresholdLevelIncreased)
		testEventMock.Expect("BranchConfirmed", testFramework.BranchID("Message7"), 0, events.ThresholdLevelDecreased)
		testEventMock.Expect("BranchConfirmed", testFramework.BranchID("Message6"), 0, events.ThresholdLevelDecreased)

		issueAndValidateMessageApproval(t, "Message11", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"):  0.55,
			testFramework.BranchID("Message6"):  0.45,
			testFramework.BranchID("Message7"):  0.25,
			testFramework.BranchID("Message11"): 0.30,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 1,
			*markers.NewMarker(1, 5): 0.55,
			*markers.NewMarker(2, 5): 0.45,
			*markers.NewMarker(2, 6): 0.35,
			*markers.NewMarker(2, 7): 0.15,
			*markers.NewMarker(2, 8): 0.15,
			*markers.NewMarker(3, 6): 0.30,
		})
	}

	// ISSUE Message12
	{
		testFramework.CreateMessage("Message12", WithStrongParents("Message11"), WithIssuer(nodes["D"].PublicKey()))

		testEventMock.Expect("BranchConfirmed", testFramework.BranchID("Message5"), 1, events.ThresholdLevelIncreased)
		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(1, 5), 1, events.ThresholdLevelIncreased)

		issueAndValidateMessageApproval(t, "Message12", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"):  0.75,
			testFramework.BranchID("Message6"):  0.25,
			testFramework.BranchID("Message7"):  0.25,
			testFramework.BranchID("Message11"): 0.50,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 1,
			*markers.NewMarker(1, 5): 0.75,
			*markers.NewMarker(2, 5): 0.25,
			*markers.NewMarker(2, 6): 0.15,
			*markers.NewMarker(2, 7): 0.15,
			*markers.NewMarker(2, 8): 0.15,
			*markers.NewMarker(3, 6): 0.50,
			*markers.NewMarker(3, 7): 0.20,
		})
	}

	// ISSUE Message13
	{
		testFramework.CreateMessage("Message13", WithStrongParents("Message12"), WithIssuer(nodes["E"].PublicKey()))

		issueAndValidateMessageApproval(t, "Message13", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"):  0.85,
			testFramework.BranchID("Message6"):  0.15,
			testFramework.BranchID("Message7"):  0.25,
			testFramework.BranchID("Message11"): 0.60,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 1,
			*markers.NewMarker(1, 5): 0.85,
			*markers.NewMarker(2, 5): 0.15,
			*markers.NewMarker(2, 6): 0.15,
			*markers.NewMarker(2, 7): 0.15,
			*markers.NewMarker(2, 8): 0.15,
			*markers.NewMarker(3, 6): 0.60,
			*markers.NewMarker(3, 7): 0.30,
			*markers.NewMarker(3, 8): 0.10,
		})
	}

	// ISSUE Message14
	{
		testFramework.CreateMessage("Message14", WithStrongParents("Message13"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("BranchConfirmed", testFramework.BranchID("Message11"), 1, events.ThresholdLevelIncreased)
		testEventMock.Expect("MarkerConfirmed", *markers.NewMarker(3, 6), 1, events.ThresholdLevelIncreased)

		issueAndValidateMessageApproval(t, "Message14", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"):  1,
			testFramework.BranchID("Message6"):  0,
			testFramework.BranchID("Message7"):  0.25,
			testFramework.BranchID("Message11"): 0.75,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 1,
			*markers.NewMarker(1, 5): 1,
			*markers.NewMarker(2, 5): 0,
			*markers.NewMarker(2, 6): 0,
			*markers.NewMarker(2, 7): 0,
			*markers.NewMarker(2, 8): 0,
			*markers.NewMarker(3, 6): 0.75,
			*markers.NewMarker(3, 7): 0.45,
			*markers.NewMarker(3, 8): 0.25,
			*markers.NewMarker(3, 9): 0.15,
		})
	}

	// ISSUE Message15
	{
		testFramework.CreateMessage("Message15", WithStrongParents("Message11", "Message5"), WithIssuer(nodes["B"].PublicKey()), WithInputs("D"), WithOutput("E", 500))

		issueAndValidateMessageApproval(t, "Message15", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"):  1,
			testFramework.BranchID("Message6"):  0,
			testFramework.BranchID("Message7"):  0.25,
			testFramework.BranchID("Message11"): 0.75,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 1,
			*markers.NewMarker(1, 5): 1,
			*markers.NewMarker(2, 5): 0,
			*markers.NewMarker(2, 6): 0,
			*markers.NewMarker(2, 7): 0,
			*markers.NewMarker(2, 8): 0,
			*markers.NewMarker(3, 6): 0.75,
			*markers.NewMarker(3, 7): 0.45,
			*markers.NewMarker(3, 8): 0.25,
			*markers.NewMarker(3, 9): 0.15,
		})
	}

	// ISSUE Message16
	{
		testFramework.CreateMessage("Message16", WithStrongParents("Message12", "Message5"), WithIssuer(nodes["B"].PublicKey()), WithInputs("D"), WithOutput("E", 500))
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message15")), "Branch4")
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message16")), "Branch5")

		issueAndValidateMessageApproval(t, "Message16", testEventMock, testFramework, map[ledgerstate.BranchID]float64{
			testFramework.BranchID("Message5"):  1,
			testFramework.BranchID("Message6"):  0,
			testFramework.BranchID("Message7"):  0.25,
			testFramework.BranchID("Message11"): 0.75,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 1,
			*markers.NewMarker(1, 5): 1,
			*markers.NewMarker(2, 5): 0,
			*markers.NewMarker(2, 6): 0,
			*markers.NewMarker(2, 7): 0,
			*markers.NewMarker(2, 8): 0,
			*markers.NewMarker(3, 6): 0.75,
			*markers.NewMarker(3, 7): 0.45,
			*markers.NewMarker(3, 8): 0.25,
			*markers.NewMarker(3, 9): 0.15,
		})
	}

	fmt.Println(testFramework.MessageMetadata("Message15"))
	fmt.Println(testFramework.MessageMetadata("Message16"))
}

func TestAggregatedBranchApproval(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	manager := epochs.NewManager(epochs.ManaRetriever(func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)))
	defer tangle.Shutdown()
	tangle.Setup()

	//testEventMock := newEventMock(t, tangle.ApprovalWeightManager)
	testFramework := NewMessageTestFramework(tangle, WithGenesisOutput("G1", 500), WithGenesisOutput("G2", 500))

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()), WithInputs("G1"), WithOutput("A", 500))
		testFramework.IssueMessages("Message1").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message1")), "Branch1")
		fmt.Println(testFramework.MessageMetadata("Message1"))
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()), WithInputs("G2"), WithOutput("B", 500))
		testFramework.IssueMessages("Message2").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message2")), "Branch2")

		fmt.Println(testFramework.MessageMetadata("Message2"))
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("C", 500))
		testFramework.IssueMessages("Message3").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message3")), "Branch3")

		fmt.Println(testFramework.MessageMetadata("Message3"))
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message2"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
		testFramework.IssueMessages("Message4").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message4")), "Branch4")

		fmt.Println(testFramework.MessageMetadata("Message4"))
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4", "Message1"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("E", 500))
		testFramework.IssueMessages("Message5").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message5")), "Branch5")
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewAggregatedBranch(ledgerstate.BranchIDs{
			testFramework.BranchID("Message4"): types.Void,
			testFramework.BranchID("Message5"): types.Void}).ID(),
			"Branch4+5",
		)

		fmt.Println(testFramework.MessageMetadata("Message5"))
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message4", "Message1"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("F", 500))
		testFramework.IssueMessages("Message6").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message6")), "Branch6")

		fmt.Println(testFramework.MessageMetadata("Message6"))
		branchID, err := tangle.Booker.MessageBranchID(testFramework.Message("Message6").ID())
		require.NoError(t, err)
		tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
			fmt.Println(branch)
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("E"), WithOutput("H", 500))
		testFramework.IssueMessages("Message7").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message7")), "Branch7")
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewAggregatedBranch(
			ledgerstate.BranchIDs{
				testFramework.BranchID("Message4"): types.Void,
				testFramework.BranchID("Message5"): types.Void,
				testFramework.BranchID("Message7"): types.Void,
			}).ID(),
			"Branch4+5+7",
		)

		fmt.Println(testFramework.MessageMetadata("Message7"))
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("E"), WithOutput("I", 500))
		testFramework.IssueMessages("Message8").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message8")), "Branch8")
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewAggregatedBranch(
			ledgerstate.BranchIDs{
				testFramework.BranchID("Message4"): types.Void,
				testFramework.BranchID("Message5"): types.Void,
				testFramework.BranchID("Message8"): types.Void,
			}).ID(),
			"Branch4+5+8",
		)
		fmt.Println(testFramework.MessageMetadata("Message8"))
		branchID, err := tangle.Booker.MessageBranchID(testFramework.Message("Message8").ID())
		require.NoError(t, err)
		tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
			fmt.Println(branch)
		})
	}
}

func issueAndValidateMessageApproval(t *testing.T, messageAlias string, eventMock *eventMock, testFramework *MessageTestFramework, expectedBranchWeights map[ledgerstate.BranchID]float64, expectedMarkerWeights map[markers.Marker]float64) {
	eventMock.Expect("MessageProcessed", testFramework.Message(messageAlias).ID())

	t.Logf("ISSUE:\tMessageID(%s)", messageAlias)
	testFramework.IssueMessages(messageAlias).WaitApprovalWeightProcessed()

	for branchID, expectedWeight := range expectedBranchWeights {
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfBranch(branchID)
		if expectedWeight != actualWeight {
			assert.True(t, math.Abs(actualWeight-expectedWeight) < 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", branchID, actualWeight, expectedWeight)
		}
	}

	for marker, expectedWeight := range expectedMarkerWeights {
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfMarker(&marker, testFramework.Message(messageAlias).IssuingTime())
		if expectedWeight != actualWeight {
			assert.True(t, math.Abs(actualWeight-expectedWeight) < 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", marker, actualWeight, expectedWeight)
		}
	}

	eventMock.AssertExpectations(t)
}

func validateMarkerSupporters(t *testing.T, approvalWeightManager *ApprovalWeightManager, markersMap map[string]*markers.StructureDetails, expectedSupporters map[string][]*identity.Identity) {
	for markerAlias, expectedSupportersOfMarker := range expectedSupporters {
		supporters := approvalWeightManager.supportersOfMarker(markersMap[markerAlias].PastMarkers.Marker())

		assert.Equal(t, len(expectedSupportersOfMarker), supporters.Size(), "size of supporters for Marker("+markerAlias+") does not match")
		for _, supporter := range expectedSupportersOfMarker {
			assert.Equal(t, true, supporters.Has(supporter.ID()))
		}
	}
}

func approveMarkers(approvalWeightManager *ApprovalWeightManager, supporter *identity.Identity, markersToApprove ...*markers.Marker) (message *Message) {
	message = newTestDataMessagePublicKey("test", supporter.PublicKey())
	approvalWeightManager.tangle.Storage.StoreMessage(message)
	approvalWeightManager.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		messageMetadata.SetStructureDetails(&markers.StructureDetails{
			Rank:          0,
			IsPastMarker:  true,
			PastMarkers:   markers.NewMarkers(markersToApprove...),
			FutureMarkers: markers.NewMarkers(),
		})
	})

	return
}

func increaseIndexCallback(markers.SequenceID, markers.Index) bool {
	return true
}

func createBranch(t *testing.T, tangle *Tangle, branchAlias string, branchIDs map[string]ledgerstate.BranchID, parentBranchID ledgerstate.BranchID, conflictID ledgerstate.ConflictID) {
	branchID := branchIDs[branchAlias]
	cachedBranch, _, err := tangle.LedgerState.BranchDAG.CreateConflictBranch(branchID, ledgerstate.NewBranchIDs(parentBranchID), ledgerstate.NewConflictIDs(conflictID))
	require.NoError(t, err)

	cachedBranch.Release()

	ledgerstate.RegisterBranchIDAlias(branchID, branchAlias)
}

func validateStatementResults(t *testing.T, approvalWeightManager *ApprovalWeightManager, branchIDs map[string]ledgerstate.BranchID, supporter Supporter, expectedResults map[string]bool) {
	for branchIDString, expectedResult := range expectedResults {
		var actualResult bool
		supporters := approvalWeightManager.supportersOfBranch(branchIDs[branchIDString])
		if supporters != nil {
			actualResult = supporters.Has(supporter)
		}

		assert.Equalf(t, expectedResult, actualResult, "%s(%s) does not match", branchIDString, branchIDs[branchIDString])
	}
}

type eventMock struct {
	mock.Mock
	expectedEvents uint64
	calledEvents   uint64
	test           *testing.T

	attached []struct {
		*events.Event
		*events.Closure
	}
}

func newEventMock(t *testing.T, approvalWeightManager *ApprovalWeightManager) *eventMock {
	e := &eventMock{
		test: t,
	}
	e.Test(t)

	approvalWeightManager.Events.BranchConfirmation.Attach(events.NewClosure(e.BranchConfirmed))
	approvalWeightManager.Events.MarkerConfirmation.Attach(events.NewClosure(e.MarkerConfirmed))

	// attach all events
	e.attach(approvalWeightManager.Events.MessageProcessed, e.MessageProcessed)

	// assure that all available events are mocked
	numEvents := reflect.ValueOf(approvalWeightManager.Events).Elem().NumField()
	assert.Equalf(t, len(e.attached)+2, numEvents, "not all events in ApprovalWeightManager.Events have been attached")

	return e
}

func (e *eventMock) DetachAll() {
	for _, a := range e.attached {
		a.Event.Detach(a.Closure)
	}
}

func (e *eventMock) Expect(eventName string, arguments ...interface{}) {
	e.On(eventName, arguments...)
	atomic.AddUint64(&e.expectedEvents, 1)
}

func (e *eventMock) attach(event *events.Event, f interface{}) {
	closure := events.NewClosure(f)
	event.Attach(closure)
	e.attached = append(e.attached, struct {
		*events.Event
		*events.Closure
	}{event, closure})
}

func (e *eventMock) AssertExpectations(t mock.TestingT) bool {
	calledEvents := atomic.LoadUint64(&e.calledEvents)
	expectedEvents := atomic.LoadUint64(&e.expectedEvents)
	if calledEvents != expectedEvents {
		t.Errorf("number of called (%d) events is not equal to number of expected events (%d)", calledEvents, expectedEvents)
		return false
	}

	defer func() {
		e.Calls = make([]mock.Call, 0)
		e.ExpectedCalls = make([]*mock.Call, 0)
		e.expectedEvents = 0
		e.calledEvents = 0
	}()

	return e.Mock.AssertExpectations(t)
}

func (e *eventMock) BranchConfirmed(branchID ledgerstate.BranchID, newLevel int, transition events.ThresholdEventTransition) {
	e.Called(branchID, newLevel, transition)

	atomic.AddUint64(&e.calledEvents, 1)
}

func (e *eventMock) MarkerConfirmed(marker markers.Marker, newLevel int, transition events.ThresholdEventTransition) {
	e.Called(marker, newLevel, transition)

	atomic.AddUint64(&e.calledEvents, 1)
}

func (e *eventMock) MessageProcessed(messageID MessageID) {
	e.Called(messageID)

	atomic.AddUint64(&e.calledEvents, 1)
}
