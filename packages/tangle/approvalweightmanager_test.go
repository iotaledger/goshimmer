package tangle

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

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

func TestSupporterManager_updateBranchSupporters(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	manaRetrieverMock := func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			identity.NewID(keyPair.PublicKey): 100,
		}
	}
	manager := epochs.NewManager(epochs.ManaRetriever(manaRetrieverMock), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)))
	defer tangle.Shutdown()
	supporterManager := NewSupporterManager(tangle)

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

	message1 := newTestDataMessagePublicKey("test", keyPair.PublicKey)
	message2 := newTestDataMessagePublicKey("test", keyPair.PublicKey)
	// statement 2: "Branch 4.1.2"
	{
		message := message2
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement2")
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 4.1.2"])
		})
		supporterManager.updateBranchSupporters(message)

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
		validateStatementResults(t, supporterManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}

	// statement 1: "Branch 1.1 + Branch 4.1.1"
	{
		message := message1
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement1")
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 1.1 + Branch 4.1.1"])
		})
		supporterManager.updateBranchSupporters(message)

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
		validateStatementResults(t, supporterManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}

	//// statement 3: "Branch 2"
	//{
	//	message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
	//	tangle.Storage.StoreMessage(message)
	//RegisterMessageIDAlias(message.ID(), "Statement3")
	//	tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
	//		messageMetadata.SetBranchID(branchIDs["Branch 2"])
	//	})
	//	supporterManager.updateBranchSupporters(message)
	//
	//	expectedResults := map[string]bool{
	//		"Branch 1":     false,
	//		"Branch 1.1":   false,
	//		"Branch 1.2":   false,
	//		"Branch 1.3":   false,
	//		"Branch 2":     true,
	//		"Branch 3":     false,
	//		"Branch 4":     true,
	//		"Branch 4.1":   true,
	//		"Branch 4.1.1": false,
	//		"Branch 4.1.2": true,
	//		"Branch 4.2":   false,
	//	}
	//	validateStatementResults(t, supporterManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	//}
}

func TestSupporterManager_updateSequenceSupporters(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()
	supporterManager := NewSupporterManager(tangle)
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
		supporterManager.updateSequenceSupporters(approveMarkers(supporterManager, supporters["A"], markers.NewMarker(1, 3)))

		validateMarkerSupporters(t, supporterManager, markersMap, map[string][]*identity.Identity{
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
		supporterManager.updateSequenceSupporters(approveMarkers(supporterManager, supporters["A"], markers.NewMarker(1, 4), markers.NewMarker(3, 5)))

		validateMarkerSupporters(t, supporterManager, markersMap, map[string][]*identity.Identity{
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
		supporterManager.updateSequenceSupporters(approveMarkers(supporterManager, supporters["A"], markers.NewMarker(5, 8)))

		validateMarkerSupporters(t, supporterManager, markersMap, map[string][]*identity.Identity{
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
		supporterManager.updateSequenceSupporters(approveMarkers(supporterManager, supporters["B"], markers.NewMarker(2, 3)))

		validateMarkerSupporters(t, supporterManager, markersMap, map[string][]*identity.Identity{
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

func TestApprovalWeightManager_ProcessMessage(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	manaRetrieverMock := func(t time.Time) map[identity.ID]float64 {
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	manager := epochs.NewManager(epochs.ManaRetriever(manaRetrieverMock), epochs.CacheTime(0))

	tangle := New(ApprovalWeights(WeightProviderFromEpochsManager(manager)))
	defer tangle.Shutdown()
	tangle.Setup()

	eventMock := newEventMock(t, tangle.ApprovalWeightManager)

	testFramework := NewMessageTestFramework(tangle, WithGenesisOutput("A", 500))

	tangle.ApprovalWeightManager.Events.BranchConfirmed.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
		tangle.LedgerState.BranchDAG.SetBranchLiked(branchID, true)
		tangle.LedgerState.BranchDAG.SetBranchFinalized(branchID, true)
	}))

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

		issueAndValidateMessageApproval(t, "Message1", eventMock, testFramework, manager, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.30,
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuer(nodes["B"].PublicKey()))

		issueAndValidateMessageApproval(t, "Message2", eventMock, testFramework, manager, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.45,
			*markers.NewMarker(1, 2): 0.15,
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["C"].PublicKey()))

		eventMock.Expect("MarkerConfirmed", markers.NewMarker(1, 1))

		issueAndValidateMessageApproval(t, "Message3", eventMock, testFramework, manager, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.70,
			*markers.NewMarker(1, 2): 0.40,
			*markers.NewMarker(1, 3): 0.25,
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()))

		eventMock.Expect("MarkerConfirmed", markers.NewMarker(1, 2))

		issueAndValidateMessageApproval(t, "Message4", eventMock, testFramework, manager, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
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

		eventMock.Expect("MarkerConfirmed", markers.NewMarker(1, 3))
		eventMock.Expect("MarkerConfirmed", markers.NewMarker(1, 4))

		issueAndValidateMessageApproval(t, "Message5", eventMock, testFramework, manager, map[ledgerstate.BranchID]float64{}, map[markers.Marker]float64{
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

		issueAndValidateMessageApproval(t, "Message6", eventMock, testFramework, manager, map[ledgerstate.BranchID]float64{
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

	return

	// ISSUE Message7
	{
		fmt.Println("-----------------    Message7   --------------------------")

		testFramework.CreateMessage("Message7", WithStrongParents("Message5"), WithIssuer(nodes["C"].PublicKey()), WithInputs("B"), WithOutput("E", 500))
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message7")), "Branch3")
		testFramework.IssueMessages("Message7").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, tangle.ApprovalWeightManager,
			MessageIDs{testFramework.Message("Message7").ID()},
			[]*markers.Marker{},
			[]ledgerstate.BranchID{},
			func() {
				tangle.ApprovalWeightManager.ProcessMessage(testFramework.Message("Message7").ID())
			},
		)
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message6"), WithIssuer(nodes["D"].PublicKey()))
		testFramework.IssueMessages("Message8").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, tangle.ApprovalWeightManager,
			MessageIDs{testFramework.Message("Message8").ID()},
			[]*markers.Marker{},
			[]ledgerstate.BranchID{},
			func() {
				tangle.ApprovalWeightManager.ProcessMessage(testFramework.Message("Message8").ID())
			},
		)
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message8"), WithIssuer(nodes["A"].PublicKey()))
		testFramework.IssueMessages("Message9").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, tangle.ApprovalWeightManager,
			MessageIDs{testFramework.Message("Message9").ID()},
			[]*markers.Marker{},
			[]ledgerstate.BranchID{},
			func() {
				tangle.ApprovalWeightManager.ProcessMessage(testFramework.Message("Message9").ID())
			},
		)
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithIssuer(nodes["B"].PublicKey()))
		testFramework.IssueMessages("Message10").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, tangle.ApprovalWeightManager,
			MessageIDs{testFramework.Message("Message10").ID()},
			[]*markers.Marker{markers.NewMarker(2, 5), markers.NewMarker(2, 6)},
			[]ledgerstate.BranchID{ledgerstate.NewBranchID(testFramework.TransactionID("Message6"))},
			func() {
				tangle.ApprovalWeightManager.ProcessMessage(testFramework.Message("Message10").ID())
			},
		)
	}

	// ISSUE Message11
	{
		tangle.LedgerState.BranchDAG.SetBranchFinalized(testFramework.BranchID("Message5"), false)
		tangle.LedgerState.BranchDAG.SetBranchFinalized(testFramework.BranchID("Message6"), false)

		fmt.Println("-----------------    Message11   --------------------------")
		testFramework.CreateMessage("Message11", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message11")), "Branch4")
		testFramework.IssueMessages("Message11").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, tangle.ApprovalWeightManager,
			MessageIDs{testFramework.Message("Message11").ID()},
			[]*markers.Marker{},
			[]ledgerstate.BranchID{},
			func() {
				tangle.ApprovalWeightManager.ProcessMessage(testFramework.Message("Message11").ID())
			},
		)
	}

	// ISSUE Message12
	{
		fmt.Println("-----------------    Message12   --------------------------")
		testFramework.CreateMessage("Message12", WithStrongParents("Message11"), WithIssuer(nodes["D"].PublicKey()))
		testFramework.IssueMessages("Message12").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, tangle.ApprovalWeightManager,
			MessageIDs{testFramework.Message("Message12").ID()},
			[]*markers.Marker{markers.NewMarker(1, 5)},
			[]ledgerstate.BranchID{ledgerstate.NewBranchID(testFramework.TransactionID("Message5"))},
			func() {
				tangle.ApprovalWeightManager.ProcessMessage(testFramework.Message("Message12").ID())
			},
		)
	}

}

func issueAndValidateMessageApproval(t *testing.T, messageAlias string, eventMock *eventMock, testFramework *MessageTestFramework, epochsManager *epochs.Manager, expectedBranchWeights map[ledgerstate.BranchID]float64, expectedMarkerWeights map[markers.Marker]float64) {
	eventMock.Expect("MessageProcessed", testFramework.Message(messageAlias).ID())

	t.Logf("MESSAGE ISSUED:\t%s", messageAlias)
	testFramework.IssueMessages(messageAlias).WaitApprovalWeightProcessed()

	for branchID, expectedWeight := range expectedBranchWeights {
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfBranch(branchID, uint64(epochsManager.TimeToOracleEpochID(testFramework.Message(messageAlias).IssuingTime())))
		assert.InEpsilonf(t, expectedWeight, actualWeight, 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", branchID, actualWeight, expectedWeight)
	}

	for marker, expectedWeight := range expectedMarkerWeights {
		actualWeight := testFramework.tangle.ApprovalWeightManager.WeightOfMarker(&marker, testFramework.Message(messageAlias).IssuingTime())
		assert.InEpsilon(t, expectedWeight, actualWeight, 0.001, "weight of %s (%0.2f) not equal to expected value %0.2f", marker, actualWeight, expectedWeight)
	}

	eventMock.AssertExpectations(t)
	eventMock.Calls = make([]mock.Call, 0)
	eventMock.ExpectedCalls = make([]*mock.Call, 0)
	eventMock.expectedEvents = 0
	eventMock.calledEvents = 0
}

func validateApprovalWeightManagerEvents(t *testing.T, approvalWeightManager *ApprovalWeightManager, expectedProcessedMessageIDs MessageIDs, expectedConfirmedMarkers []*markers.Marker, expectedConfirmedBranches []ledgerstate.BranchID, callback func()) {
	var actualProcessedMessageIDs MessageIDs
	messageProcessedEventHandler := events.NewClosure(func(messageID MessageID) {
		actualProcessedMessageIDs = append(actualProcessedMessageIDs, messageID)
	})
	approvalWeightManager.Events.MessageProcessed.Attach(messageProcessedEventHandler)

	var actualConfirmedMarkers []*markers.Marker
	markerConfirmedEventHandler := events.NewClosure(func(marker *markers.Marker) {
		actualConfirmedMarkers = append(actualConfirmedMarkers, marker)
	})
	approvalWeightManager.Events.MarkerConfirmed.Attach(markerConfirmedEventHandler)

	var actualConfirmedBranches []ledgerstate.BranchID
	branchConfirmedEventHandler := events.NewClosure(func(branchID ledgerstate.BranchID) {
		actualConfirmedBranches = append(actualConfirmedBranches, branchID)
	})
	approvalWeightManager.Events.BranchConfirmed.Attach(branchConfirmedEventHandler)

	callback()

	assert.ElementsMatch(t, expectedProcessedMessageIDs, actualProcessedMessageIDs)
	assert.ElementsMatch(t, expectedConfirmedMarkers, actualConfirmedMarkers)
	assert.ElementsMatch(t, expectedConfirmedBranches, actualConfirmedBranches)

	approvalWeightManager.Events.MessageProcessed.Detach(messageProcessedEventHandler)
	approvalWeightManager.Events.MarkerConfirmed.Detach(markerConfirmedEventHandler)
	approvalWeightManager.Events.BranchConfirmed.Detach(branchConfirmedEventHandler)
}

func validateMarkerSupporters(t *testing.T, approvalWeightManager *SupporterManager, markersMap map[string]*markers.StructureDetails, expectedSupporters map[string][]*identity.Identity) {
	for markerAlias, expectedSupportersOfMarker := range expectedSupporters {
		supporters := approvalWeightManager.SupportersOfMarker(markersMap[markerAlias].PastMarkers.HighestSequenceMarker())

		assert.Equal(t, len(expectedSupportersOfMarker), supporters.Size(), "size of supporters for Marker("+markerAlias+") does not match")
		for _, supporter := range expectedSupportersOfMarker {
			assert.Equal(t, true, supporters.Has(supporter.ID()))
		}
	}
}

func approveMarkers(approvalWeightManager *SupporterManager, supporter *identity.Identity, markersToApprove ...*markers.Marker) (message *Message) {
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

func validateStatementResults(t *testing.T, approvalWeightManager *SupporterManager, branchIDs map[string]ledgerstate.BranchID, supporter Supporter, expectedResults map[string]bool) {
	for branchIDString, expectedResult := range expectedResults {
		var actualResult bool
		supporters := approvalWeightManager.branchSupporters[branchIDs[branchIDString]]
		if supporters != nil {
			actualResult = supporters.Has(supporter)
		}

		assert.Equalf(t, expectedResult, actualResult, "%s(%s) does not match", branchIDString, branchIDs[branchIDString])
	}
}

type eventMock struct {
	mock.Mock
	expectedEvents int
	calledEvents   int
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

	// attach all events
	e.attach(approvalWeightManager.Events.BranchConfirmed, e.BranchConfirmed)
	e.attach(approvalWeightManager.Events.MarkerConfirmed, e.MarkerConfirmed)
	e.attach(approvalWeightManager.Events.MessageProcessed, e.MessageProcessed)

	// assure that all available events are mocked
	numEvents := reflect.ValueOf(approvalWeightManager.Events).Elem().NumField()
	assert.Equalf(t, len(e.attached), numEvents, "not all events in ApprovalWeightManager.Events have been attached")

	return e
}

func (e *eventMock) DetachAll() {
	for _, a := range e.attached {
		a.Event.Detach(a.Closure)
	}
}

func (e *eventMock) Expect(eventName string, arguments ...interface{}) {
	e.On(eventName, arguments...)
	e.expectedEvents++
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
	if e.calledEvents != e.expectedEvents {
		t.Errorf("number of called (%d) events is not equal to number of expected events (%d)", e.calledEvents, e.expectedEvents)
		return false
	}

	return e.Mock.AssertExpectations(t)
}

func (e *eventMock) BranchConfirmed(branchID ledgerstate.BranchID) {
	e.test.Logf("EVENT TRIGGERED:\tBranchConfirmed(%s)", branchID)

	e.Called(branchID)

	e.calledEvents++
}

func (e *eventMock) MarkerConfirmed(marker *markers.Marker) {
	e.test.Logf("EVENT TRIGGERED:\tMarkerConfirmed(%s)", marker)

	e.Called(marker)

	e.calledEvents++
}

func (e *eventMock) MessageProcessed(messageID MessageID) {
	e.test.Logf("EVENT TRIGGERED:\tMessageProcessed(%s)", messageID)

	e.Called(messageID)

	e.calledEvents++
}
