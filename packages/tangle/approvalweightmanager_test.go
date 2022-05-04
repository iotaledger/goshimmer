//nolint:dupl
package tangle

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/thresholdmap"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func BenchmarkApprovalWeightManager_ProcessMessage_Conflicts(b *testing.B) {
	voters := map[string]*identity.Identity{
		"A": identity.New(ed25519.GenerateKeyPair().PublicKey),
		"B": identity.New(ed25519.GenerateKeyPair().PublicKey),
	}
	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		m := make(map[identity.ID]float64)
		for _, s := range voters {
			weightProvider.Update(time.Now(), s.ID())
			m[s.ID()] = 100
		}
		return m
	}
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now)

	tangle := NewTestTangle(ApprovalWeights(weightProvider))
	defer tangle.Shutdown()
	approvalWeightManager := tangle.ApprovalWeightManager

	// build markers DAG where each sequence has only 1 marker building a chain of sequences
	totalMarkers := 10000
	{
		var previousMarker *markers.StructureDetails
		for i := uint32(1); i < uint32(totalMarkers); i++ {
			if previousMarker == nil {
				previousMarker, _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails(nil, increaseIndexCallback)
				continue
			}

			previousMarker, _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{previousMarker}, increaseIndexCallback)
		}
	}

	// measure time for each marker
	for i := 1; i < 3; i++ {
		measurements := 100
		var total time.Duration
		for m := 0; m < measurements; m++ {
			start := time.Now()
			approvalWeightManager.updateSequenceVoters(approveMarkers(approvalWeightManager, voters["A"], markers.NewMarker(markers.SequenceID(i), markers.Index(i))))
			total += time.Since(start)
		}
	}
}

func TestBranchWeightMarshalling(t *testing.T) {
	branchWeight := NewBranchWeight(ledgerstate.BranchIDFromRandomness())
	branchWeight.SetWeight(5.1234)

	branchWeightFromBytes, err := new(BranchWeight).FromBytes(branchWeight.Bytes())
	require.NoError(t, err)

	assert.Equal(t, branchWeight.Bytes(), branchWeightFromBytes.Bytes())
	assert.Equal(t, branchWeight.BranchID(), branchWeightFromBytes.BranchID())
	assert.Equal(t, branchWeight.Weight(), branchWeightFromBytes.Weight())
}

func TestBranchVotersMarshalling(t *testing.T) {
	branchVoters := NewBranchVoters(ledgerstate.BranchIDFromRandomness())

	for i := 0; i < 100; i++ {
		branchVoters.AddVoter(identity.GenerateIdentity().ID())
	}

	branchVotersFromBytes, err := new(BranchVoters).FromBytes(branchVoters.Bytes())
	require.NoError(t, err)

	// verify that branchVotersFromBytes has all voters from branchVoters
	assert.Equal(t, branchVoters.Voters().Set.Size(), branchVotersFromBytes.Voters().Set.Size())
	branchVoters.Voters().Set.ForEach(func(voter Voter) {
		assert.True(t, branchVotersFromBytes.Voters().Set.Has(voter))
	})
}

// TestApprovalWeightManager_updateBranchVoters tests the ApprovalWeightManager's functionality regarding branches.
// The scenario can be found in images/approvalweight-updateBranchSupporters.png.
func TestApprovalWeightManager_updateBranchVoters(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		nodeID := identity.NewID(keyPair.PublicKey)
		weightProvider.Update(time.Now(), nodeID)
		return map[identity.ID]float64{
			nodeID: 100,
		}
	}
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now)

	tangle := NewTestTangle(ApprovalWeights(weightProvider))
	defer tangle.Shutdown()
	approvalWeightManager := tangle.ApprovalWeightManager
	tangle.Configure(MergeBranches(false))

	conflictIDs := map[string]ledgerstate.ConflictID{
		"Conflict 1": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 2": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 3": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 4": ledgerstate.ConflictIDFromRandomness(),
		"Conflict 5": ledgerstate.ConflictIDFromRandomness(),
	}

	branchIDs := map[string]ledgerstate.BranchIDs{
		"Branch 1":     ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 1.1":   ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 1.2":   ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 1.3":   ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 2":     ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 3":     ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 4":     ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 4.1":   ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 4.1.1": ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 4.1.2": ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
		"Branch 4.2":   ledgerstate.NewBranchIDs(ledgerstate.BranchIDFromRandomness()),
	}

	createBranch(t, tangle, "Branch 1", branchIDs, ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), conflictIDs["Conflict 1"])
	createBranch(t, tangle, "Branch 2", branchIDs, ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), conflictIDs["Conflict 1"])
	createBranch(t, tangle, "Branch 3", branchIDs, ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), conflictIDs["Conflict 2"])
	createBranch(t, tangle, "Branch 4", branchIDs, ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), conflictIDs["Conflict 2"])

	createBranch(t, tangle, "Branch 1.1", branchIDs, branchIDs["Branch 1"], conflictIDs["Conflict 3"])
	createBranch(t, tangle, "Branch 1.2", branchIDs, branchIDs["Branch 1"], conflictIDs["Conflict 3"])
	createBranch(t, tangle, "Branch 1.3", branchIDs, branchIDs["Branch 1"], conflictIDs["Conflict 3"])

	createBranch(t, tangle, "Branch 4.1", branchIDs, branchIDs["Branch 4"], conflictIDs["Conflict 4"])
	createBranch(t, tangle, "Branch 4.2", branchIDs, branchIDs["Branch 4"], conflictIDs["Conflict 4"])

	createBranch(t, tangle, "Branch 4.1.1", branchIDs, branchIDs["Branch 4.1"], conflictIDs["Conflict 5"])
	createBranch(t, tangle, "Branch 4.1.2", branchIDs, branchIDs["Branch 4.1"], conflictIDs["Conflict 5"])

	branchIDs["Branch 1.1 + Branch 4.1.1"] = ledgerstate.NewBranchIDs().AddAll(branchIDs["Branch 1.1"]).AddAll(branchIDs["Branch 4.1.1"])

	// Issue statements in different order to make sure that no information is lost when nodes apply statements in arbitrary order

	message1 := newTestDataMessagePublicKey("test1", keyPair.PublicKey)
	message2 := newTestDataMessagePublicKey("test2", keyPair.PublicKey)
	// statement 2: "Branch 4.1.2"
	{
		message := message2
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement2")
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetAddedBranchIDs(branchIDs["Branch 4.1.2"])
			messageMetadata.SetStructureDetails(&markers.StructureDetails{
				Rank:          0,
				IsPastMarker:  false,
				PastMarkers:   markers.NewMarkers(),
				FutureMarkers: markers.NewMarkers(),
			})
		})
		approvalWeightManager.updateBranchVoters(message)

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
			messageMetadata.SetAddedBranchIDs(branchIDs["Branch 1.1 + Branch 4.1.1"])
			messageMetadata.SetStructureDetails(&markers.StructureDetails{
				Rank:          0,
				IsPastMarker:  false,
				PastMarkers:   markers.NewMarkers(),
				FutureMarkers: markers.NewMarkers(),
			})
		})
		approvalWeightManager.updateBranchVoters(message)

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

	// statement 3: "Branch 2"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement3")
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetAddedBranchIDs(branchIDs["Branch 2"])
			messageMetadata.SetStructureDetails(&markers.StructureDetails{
				Rank:          0,
				IsPastMarker:  false,
				PastMarkers:   markers.NewMarkers(),
				FutureMarkers: markers.NewMarkers(),
			})
		})
		approvalWeightManager.updateBranchVoters(message)

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

// TestApprovalWeightManager_updateSequenceVoters tests the ApprovalWeightManager's functionality regarding sequences.
// The scenario can be found in images/approvalweight-updateSequenceSupporters.png.
func TestApprovalWeightManager_updateSequenceVoters(t *testing.T) {
	voters := map[string]*identity.Identity{
		"A": identity.New(ed25519.GenerateKeyPair().PublicKey),
		"B": identity.New(ed25519.GenerateKeyPair().PublicKey),
	}
	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		m := make(map[identity.ID]float64)
		for _, s := range voters {
			weightProvider.Update(time.Now(), s.ID())
			m[s.ID()] = 100
		}
		return m
	}
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now)

	tangle := NewTestTangle(ApprovalWeights(weightProvider))
	defer tangle.Shutdown()
	approvalWeightManager := tangle.ApprovalWeightManager

	markersMap := make(map[string]*markers.StructureDetails)

	// build markers DAG
	{
		markersMap["0,1"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails(nil, increaseIndexCallback)
		markersMap["0,2"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,1"]}, increaseIndexCallback)
		markersMap["0,3"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,2"]}, increaseIndexCallback)
		markersMap["0,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,3"]}, increaseIndexCallback)

		markersMap["0,1"].PastMarkerGap = 50
		markersMap["1,2"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,1"]}, increaseIndexCallback)
		markersMap["1,3"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,2"]}, increaseIndexCallback)
		markersMap["1,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,3"]}, increaseIndexCallback)
		markersMap["1,5"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,4"]}, increaseIndexCallback)

		markersMap["0,3"].PastMarkerGap = 50
		markersMap["1,4"].PastMarkerGap = 50
		markersMap["2,5"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,3"], markersMap["1,4"]}, increaseIndexCallback)
		markersMap["2,6"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,4"], markersMap["2,5"]}, increaseIndexCallback)
		markersMap["2,7"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,6"]}, increaseIndexCallback)
		markersMap["2,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,7"]}, increaseIndexCallback)

		markersMap["2,7"].PastMarkerGap = 50
		markersMap["3,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,7"]}, increaseIndexCallback)
		markersMap["1,4"].PastMarkerGap = 50
		markersMap["4,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,7"], markersMap["1,4"]}, increaseIndexCallback)
	}

	// CASE1: APPROVE MARKER(0, 3)
	{
		approvalWeightManager.updateSequenceVoters(approveMarkers(approvalWeightManager, voters["A"], markers.NewMarker(0, 3)))

		validateMarkerVoters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"0,1": {voters["A"]},
			"0,2": {voters["A"]},
			"0,3": {voters["A"]},
			"0,4": {},
			"1,2": {},
			"1,3": {},
			"1,4": {},
			"1,5": {},
			"2,5": {},
			"2,6": {},
			"2,7": {},
			"2,8": {},
			"3,8": {},
			"4,8": {},
		})
	}

	// CASE2: APPROVE MARKER(0, 4) + MARKER(2, 6)
	{
		approvalWeightManager.updateSequenceVoters(approveMarkers(approvalWeightManager, voters["A"], markers.NewMarker(0, 4), markers.NewMarker(2, 6)))

		validateMarkerVoters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"0,1": {voters["A"]},
			"0,2": {voters["A"]},
			"0,3": {voters["A"]},
			"0,4": {voters["A"]},
			"1,2": {voters["A"]},
			"1,3": {voters["A"]},
			"1,4": {voters["A"]},
			"1,5": {},
			"2,5": {voters["A"]},
			"2,6": {voters["A"]},
			"2,7": {},
			"2,8": {},
			"3,8": {},
			"4,8": {},
		})
	}

	// CASE3: APPROVE MARKER(4, 8)
	{
		approvalWeightManager.updateSequenceVoters(approveMarkers(approvalWeightManager, voters["A"], markers.NewMarker(4, 8)))

		validateMarkerVoters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"0,1": {voters["A"]},
			"0,2": {voters["A"]},
			"0,3": {voters["A"]},
			"0,4": {voters["A"]},
			"1,2": {voters["A"]},
			"1,3": {voters["A"]},
			"1,4": {voters["A"]},
			"1,5": {},
			"2,5": {voters["A"]},
			"2,6": {voters["A"]},
			"2,7": {voters["A"]},
			"2,8": {},
			"3,8": {},
			"4,8": {voters["A"]},
		})
	}

	// CASE4: APPROVE MARKER(1, 5)
	{
		approvalWeightManager.updateSequenceVoters(approveMarkers(approvalWeightManager, voters["B"], markers.NewMarker(1, 5)))

		validateMarkerVoters(t, approvalWeightManager, markersMap, map[string][]*identity.Identity{
			"0,1": {voters["A"], voters["B"]},
			"0,2": {voters["A"]},
			"0,3": {voters["A"]},
			"0,4": {voters["A"]},
			"1,2": {voters["A"], voters["B"]},
			"1,3": {voters["A"], voters["B"]},
			"1,4": {voters["A"], voters["B"]},
			"1,5": {voters["B"]},
			"2,5": {voters["A"]},
			"2,6": {voters["A"]},
			"2,7": {voters["A"]},
			"2,8": {},
			"3,8": {},
			"4,8": {voters["A"]},
		})
	}
}

// TestApprovalWeightManager_ProcessMessage tests the whole functionality of the ApprovalWeightManager.
// The scenario can be found in images/approvalweight-processMessage.png.
func TestApprovalWeightManager_ProcessMessage(t *testing.T) {
	processMsgScenario := ProcessMessageScenario(t)
	defer func(processMsgScenario *TestScenario, t *testing.T) {
		if err := processMsgScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processMsgScenario, t)

	for processMsgScenario.HasNext() {
		processMsgScenario.Next(nil)
	}
}

func TestAggregatedBranchApproval(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now)

	tangle := NewTestTangle(ApprovalWeights(weightProvider))
	defer tangle.Shutdown()
	tangle.Setup()

	testFramework := NewMessageTestFramework(tangle, WithGenesisOutput("G1", 500), WithGenesisOutput("G2", 500))

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()), WithInputs("G1"), WithOutput("A", 500))
		testFramework.IssueMessages("Message1").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message1")), "Branch1")
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()), WithInputs("G2"), WithOutput("B", 500))
		testFramework.IssueMessages("Message2").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message2")), "Branch2")
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("C", 500))
		testFramework.IssueMessages("Message3").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message3")), "Branch3")
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message2"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
		testFramework.IssueMessages("Message4").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message4")), "Branch4")
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4", "Message1"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("E", 500))
		testFramework.IssueMessages("Message5").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message5")), "Branch5")
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message4", "Message1"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("F", 500))
		testFramework.IssueMessages("Message6").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message6")), "Branch6")

		_, err := tangle.Booker.MessageBranchIDs(testFramework.Message("Message6").ID())
		require.NoError(t, err)
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("E"), WithOutput("H", 500))
		testFramework.IssueMessages("Message7").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message7")), "Branch7")
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("E"), WithOutput("I", 500))
		testFramework.IssueMessages("Message8").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message8")), "Branch8")
		_, err := tangle.Booker.MessageBranchIDs(testFramework.Message("Message8").ID())
		require.NoError(t, err)
	}
}

func TestOutOfOrderStatements(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
		identity.RegisterIDAlias(nodes[node].ID(), node)
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			weightProvider.Update(time.Now(), node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now)

	tangle := NewTestTangle(ApprovalWeights(weightProvider))
	tangle.Configure(MergeBranches(false))
	tangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 3

	tangle.Setup()
	testEventMock := NewEventMock(t, tangle.ApprovalWeightManager)
	testFramework := NewMessageTestFramework(tangle, WithGenesisOutput("A", 500), WithGenesisOutput("B", 500))

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.3)

		IssueAndValidateMessageApproval(t, "Message1", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 0.30,
		})
	}
	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.45)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.15)

		IssueAndValidateMessageApproval(t, "Message2", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 0.45,
			*markers.NewMarker(0, 2): 0.15,
		})
	}
	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["C"].PublicKey()), WithInputs("A"), WithOutput("A3", 500))
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.70)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.40)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.25)

		IssueAndValidateMessageApproval(t, "Message3", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 0.70,
			*markers.NewMarker(0, 2): 0.40,
			*markers.NewMarker(0, 3): 0.25,
		})
	}
	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.90)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.60)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.45)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.20)

		IssueAndValidateMessageApproval(t, "Message4", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 0.90,
			*markers.NewMarker(0, 2): 0.60,
			*markers.NewMarker(0, 3): 0.45,
			*markers.NewMarker(0, 4): 0.20,
		})
	}
	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A3"), WithOutput("A5", 500))
		testFramework.RegisterBranchID("A", "Message5")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.90)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.50)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.30)

		IssueAndValidateMessageApproval(t, "Message5", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 0.90,
			*markers.NewMarker(0, 2): 0.90,
			*markers.NewMarker(0, 3): 0.75,
			*markers.NewMarker(0, 4): 0.50,
			*markers.NewMarker(0, 5): 0.30,
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message4"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A3"), WithOutput("B6", 500))
		testFramework.RegisterBranchID("B", "Message6")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.85)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.60)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("A"), 0.30)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.10)

		IssueAndValidateMessageApproval(t, "Message6", testEventMock, testFramework, map[string]float64{
			"A": 0.3,
			"B": 0.1,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 0.85,
			*markers.NewMarker(0, 4): 0.60,
			*markers.NewMarker(0, 5): 0.30,
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message3"), WithIssuer(nodes["B"].PublicKey()), WithInputs("B"), WithOutput("B7", 500))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 1.0)

		IssueAndValidateMessageApproval(t, "Message7", testEventMock, testFramework, map[string]float64{
			"A": 0.30,
			"B": 0.1,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 1,
			*markers.NewMarker(0, 4): 0.60,
			*markers.NewMarker(0, 5): 0.30,
		})
	}
	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()), WithInputs("B"), WithOutput("B8", 500))
		testFramework.RegisterBranchID("C", "Message7")
		testFramework.RegisterBranchID("D", "Message8")

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("C"), 0.15)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("D"), 0.20)

		IssueAndValidateMessageApproval(t, "Message8", testEventMock, testFramework, map[string]float64{
			"A": 0.30,
			"B": 0.10,
			"C": 0.15,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 1,
			*markers.NewMarker(0, 4): 0.60,
			*markers.NewMarker(0, 5): 0.30,
		})
	}
	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message6", "Message7"), WithIssuer(nodes["A"].PublicKey()))

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("A"), 0.0)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.40)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("C"), 0.45)

		IssueAndValidateMessageApproval(t, "Message9", testEventMock, testFramework, map[string]float64{
			"A": 0,
			"B": 0.40,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 1,
			*markers.NewMarker(0, 4): 0.60,
			*markers.NewMarker(0, 5): 0.30,
		})
	}
	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.15)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.55)

		IssueAndValidateMessageApproval(t, "Message10", testEventMock, testFramework, map[string]float64{
			"A": 0,
			"B": 0.55,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 1,
			*markers.NewMarker(0, 4): 0.75,
			*markers.NewMarker(0, 5): 0.30,
			*markers.NewMarker(1, 5): 0.15,
		})
	}
	// ISSUE Message11
	{
		// We skip ahead with the Sequence Number
		testFramework.CreateMessage("Message11", WithStrongParents("Message5"), WithIssuer(nodes["E"].PublicKey()), WithSequenceNumber(1000))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.40)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 6), 0.10)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("A"), 0.10)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.45)

		IssueAndValidateMessageApproval(t, "Message11", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 1,
			*markers.NewMarker(0, 4): 0.75,
			*markers.NewMarker(0, 5): 0.40,
			*markers.NewMarker(0, 6): 0.10,
			*markers.NewMarker(1, 5): 0.15,
		})
	}

	// ISSUE Message12
	{
		// We simulate an "old" vote
		testFramework.CreateMessage("Message12", WithStrongParents("Message10"), WithIssuer(nodes["E"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.25)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 6), 0.10)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("C"), 0.55)

		IssueAndValidateMessageApproval(t, "Message12", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.55,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 1,
			*markers.NewMarker(0, 4): 0.75,
			*markers.NewMarker(0, 5): 0.40,
			*markers.NewMarker(0, 6): 0.10,
			*markers.NewMarker(1, 5): 0.25,
			*markers.NewMarker(1, 6): 0.10,
		})
	}

	// ISSUE Message13
	{
		// We simulate an "old" vote
		testFramework.CreateMessage("Message13", WithStrongParents("Message2"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A"), WithOutput("A13", 500))
		testFramework.RegisterBranchID("X", "Message3")
		testFramework.RegisterBranchID("Y", "Message13")

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("X"), 1.0)

		IssueAndValidateMessageApproval(t, "Message13", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.55,
			"D": 0.20,
			"X": 1.00,
			"Y": 0.00,
		}, map[markers.Marker]float64{
			*markers.NewMarker(0, 1): 1,
			*markers.NewMarker(0, 2): 1,
			*markers.NewMarker(0, 3): 1,
			*markers.NewMarker(0, 4): 0.75,
			*markers.NewMarker(0, 5): 0.40,
			*markers.NewMarker(0, 6): 0.10,
			*markers.NewMarker(1, 5): 0.25,
			*markers.NewMarker(1, 6): 0.10,
		})
	}

	testEventMock.AssertExpectations(t)
}

func TestLatestMarkerVotes(t *testing.T) {
	{
		latestMarkerVotes := NewLatestMarkerVotes(1, Voter{1})
		latestMarkerVotes.Store(1, 8)
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]uint64{
			1: 8,
		})
		latestMarkerVotes.Store(2, 10)
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]uint64{
			2: 10,
		})
		latestMarkerVotes.Store(3, 7)
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]uint64{
			2: 10,
			3: 7,
		})
		latestMarkerVotes.Store(4, 9)
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]uint64{
			2: 10,
			4: 9,
		})
		latestMarkerVotes.Store(4, 11)
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]uint64{
			4: 11,
		})
		latestMarkerVotes.Store(1, 15)
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]uint64{
			1: 15,
			4: 11,
		})
	}

	{
		latestMarkerVotes := NewLatestMarkerVotes(1, Voter{1})
		latestMarkerVotes.Store(3, 7)
		latestMarkerVotes.Store(2, 10)
		latestMarkerVotes.Store(4, 9)
		latestMarkerVotes.Store(1, 8)
		latestMarkerVotes.Store(1, 15)
		latestMarkerVotes.Store(4, 11)
		validateLatestMarkerVotes(t, latestMarkerVotes, map[markers.Index]uint64{
			1: 15,
			4: 11,
		})
	}
}

func validateLatestMarkerVotes(t *testing.T, votes *LatestMarkerVotes, expectedVotes map[markers.Index]uint64) {
	votes.latestMarkerVotesInner.LatestMarkerVotes.ForEach(func(node *thresholdmap.Element[markers.Index, uint64]) bool {
		index := node.Key()
		seq := node.Value()

		_, exists := expectedVotes[index]
		assert.Truef(t, exists, "%s:%d does not exist in latestMarkerVotes", index, seq)
		delete(expectedVotes, index)

		return true
	})
	assert.Empty(t, expectedVotes)
}

func validateMarkerVoters(t *testing.T, approvalWeightManager *ApprovalWeightManager, markersMap map[string]*markers.StructureDetails, expectedVoters map[string][]*identity.Identity) {
	for markerAlias, expectedVotersOfMarker := range expectedVoters {
		// sanity check
		assert.Equal(t, markerAlias, fmt.Sprintf("%d,%d", markersMap[markerAlias].PastMarkers.Marker().SequenceID(), markersMap[markerAlias].PastMarkers.Marker().Index()))

		voters := approvalWeightManager.markerVotes(markersMap[markerAlias].PastMarkers.Marker())

		assert.Equal(t, len(expectedVotersOfMarker), len(voters), "size of voters for Marker("+markerAlias+") does not match")
		for _, voter := range expectedVotersOfMarker {
			_, voterExists := voters[voter.ID()]
			assert.True(t, voterExists)
		}
	}
}

func approveMarkers(approvalWeightManager *ApprovalWeightManager, voter *identity.Identity, markersToApprove ...*markers.Marker) (message *Message) {
	message = newTestDataMessagePublicKey("test", voter.PublicKey())
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

func getSingleBranch(branches map[string]ledgerstate.BranchIDs, alias string) ledgerstate.BranchID {
	if len(branches[alias]) != 1 {
		panic(fmt.Sprintf("Branches with alias %s are multiple branches, not a single one: %s", alias, branches[alias]))
	}

	for branchID := range branches[alias] {
		return branchID
	}

	return ledgerstate.UndefinedBranchID
}

func createBranch(t *testing.T, tangle *Tangle, branchAlias string, branchIDs map[string]ledgerstate.BranchIDs, parentBranchIDs ledgerstate.BranchIDs, conflictID ledgerstate.ConflictID) {
	branchID := getSingleBranch(branchIDs, branchAlias)
	cachedBranch, _, err := tangle.LedgerState.BranchDAG.CreateBranch(branchID, parentBranchIDs, ledgerstate.NewConflictIDs(conflictID))
	require.NoError(t, err)

	cachedBranch.Release()

	ledgerstate.RegisterBranchIDAlias(branchID, branchAlias)
}

func validateStatementResults(t *testing.T, approvalWeightManager *ApprovalWeightManager, branchIDs map[string]ledgerstate.BranchIDs, voter Voter, expectedResults map[string]bool) {
	for branchIDString, expectedResult := range expectedResults {
		var actualResult bool
		for branchID := range branchIDs[branchIDString] {
			voters := approvalWeightManager.VotersOfBranch(branchID)
			if voters != nil {
				actualResult = voters.Set.Has(voter)
			}
			if !actualResult {
				break
			}
		}

		assert.Equalf(t, expectedResult, actualResult, "%s(%s) does not match", branchIDString, branchIDs[branchIDString])
	}
}
