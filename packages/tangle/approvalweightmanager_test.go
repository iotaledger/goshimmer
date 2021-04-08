package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestApprovalWeightManager_ProcessMessage(t *testing.T) {
}

func TestSupporterManager_updateBranchSupporters(t *testing.T) {
	tangle := New()
	defer tangle.Shutdown()
	supporterManager := NewSupporterManager(tangle)

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

	cachedAggregatedBranch, _, err := tangle.LedgerState.BranchDAG.AggregateBranches(ledgerstate.NewBranchIDs(branchIDs["Branch 1.1"], branchIDs["Branch 4.1.1"]))
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
			"Branch 4.1.1": true,
			"Branch 4.1.2": false,
			"Branch 4.2":   false,
		}
		validateStatementResults(t, supporterManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}

	// statement 2: "Branch 4.1.2"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 4.1.2"])
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

	// statement 3: "Branch 2"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(branchIDs["Branch 2"])
		})
		supporterManager.updateBranchSupporters(message)

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
		validateStatementResults(t, supporterManager, branchIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}
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
	tangle := New()
	defer tangle.Shutdown()
	tangle.Setup()

	testFramework := NewMessageTestFramework(tangle)

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

	approvalWeightManager := NewApprovalWeightManager(tangle, manager)
	approvalWeightManager.Setup()

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))
		testFramework.IssueMessages("Message1").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, approvalWeightManager,
			MessageIDs{testFramework.Message("Message1").ID()},
			[]*markers.Marker{},
			[]ledgerstate.BranchID{},
			func() {
				approvalWeightManager.ProcessMessage(testFramework.Message("Message1").ID())
			},
		)
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuer(nodes["B"].PublicKey()))
		testFramework.IssueMessages("Message2").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, approvalWeightManager,
			MessageIDs{testFramework.Message("Message2").ID()},
			[]*markers.Marker{},
			[]ledgerstate.BranchID{},
			func() {
				approvalWeightManager.ProcessMessage(testFramework.Message("Message2").ID())
			},
		)
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["C"].PublicKey()))
		testFramework.IssueMessages("Message3").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, approvalWeightManager,
			MessageIDs{testFramework.Message("Message3").ID()},
			[]*markers.Marker{markers.NewMarker(1, 1)},
			[]ledgerstate.BranchID{ledgerstate.MasterBranchID},
			// TODO: handle MasterBRanchID differently?
			func() {
				approvalWeightManager.ProcessMessage(testFramework.Message("Message3").ID())
			},
		)
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()))
		testFramework.IssueMessages("Message4").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, approvalWeightManager,
			MessageIDs{testFramework.Message("Message4").ID()},
			[]*markers.Marker{markers.NewMarker(1, 2)},
			[]ledgerstate.BranchID{ledgerstate.MasterBranchID},
			func() {
				approvalWeightManager.ProcessMessage(testFramework.Message("Message4").ID())
			},
		)
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithIssuer(nodes["A"].PublicKey()))
		testFramework.IssueMessages("Message5").WaitMessagesBooked()

		validateApprovalWeightManagerEvents(t, approvalWeightManager,
			MessageIDs{testFramework.Message("Message5").ID()},
			[]*markers.Marker{markers.NewMarker(1, 3), markers.NewMarker(1, 4)},
			[]ledgerstate.BranchID{},
			func() {
				approvalWeightManager.ProcessMessage(testFramework.Message("Message5").ID())
			},
		)
	}
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

func createBranch(t *testing.T, tangle *Tangle, branchID, parentBranchID ledgerstate.BranchID, conflictID ledgerstate.ConflictID) {
	cachedBranch, _, err := tangle.LedgerState.BranchDAG.CreateConflictBranch(branchID, ledgerstate.NewBranchIDs(parentBranchID), ledgerstate.NewConflictIDs(conflictID))
	require.NoError(t, err)

	cachedBranch.Release()
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
