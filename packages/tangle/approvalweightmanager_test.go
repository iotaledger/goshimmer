package tangle

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func toByteArray(i uint32) (arr []byte) {
	arr = make([]byte, 4)
	binary.BigEndian.PutUint32(arr, i)
	return
}

func BenchmarkApprovalWeightManager_ProcessMessage_Conflicts(b *testing.B) {
	supporters := map[string]*identity.Identity{
		"A": identity.New(ed25519.GenerateKeyPair().PublicKey),
		"B": identity.New(ed25519.GenerateKeyPair().PublicKey),
	}
	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		m := make(map[identity.ID]float64)
		for _, s := range supporters {
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
				previousMarker, _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails(nil, increaseIndexCallback, markers.NewSequenceAlias(toByteArray(i)))
				continue
			}

			previousMarker, _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{previousMarker}, increaseIndexCallback, markers.NewSequenceAlias(toByteArray(i)))
		}
	}

	// measure time for each marker
	for i := 1; i < 3; i++ {
		measurements := 100
		var total time.Duration
		for m := 0; m < measurements; m++ {
			start := time.Now()
			approvalWeightManager.updateSequenceSupporters(approveMarkers(approvalWeightManager, supporters["A"], markers.NewMarker(markers.SequenceID(i), markers.Index(i))))
			total += time.Since(start)
		}
	}
}

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
	statement := NewStatement(identity.GenerateIdentity().ID())
	statement.UpdateSequenceNumber(10)
	statementFromBytes, _, err := StatementFromBytes(statement.Bytes())
	require.NoError(t, err)

	assert.Equal(t, statement.Bytes(), statementFromBytes.Bytes())
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
	branchSupporters.Supporters().ForEach(func(supporter Voter) {
		assert.True(t, branchSupportersFromBytes.supporters.Has(supporter))
	})
}

/*
func TestBlub(t *testing.T) {
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

	branches := make(map[string]ledgerstate.BranchID)
	branches["A"] := ledgerstate.BranchIDFromRandomness()
	branches["B"] := ledgerstate.BranchIDFromRandomness()

}
*/

// TestApprovalWeightManager_updateBranchSupporters tests the ApprovalWeightManager's functionality regarding branches.
// The scenario can be found in images/approvalweight-updateBranchSupporters.png.

/*
func TestApprovalWeightManager_updateBranchSupporters(t *testing.T) {
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

	aggregatedBranchID := tangle.LedgerState.BranchDAG.AggregateConflictBranchesID(ledgerstate.NewBranchIDs(branchIDs["Branch 1.1"], branchIDs["Branch 4.1.1"]))
	branchIDs["Branch 1.1 + Branch 4.1.1"] = aggregatedBranchID
	ledgerstate.RegisterBranchIDAlias(aggregatedBranchID, "Branch 1.1 + Branch 4.1.1")

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
				PastMarkers:   markers.NewMarkers(),
				FutureMarkers: markers.NewMarkers(),
			})
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
			messageMetadata.SetAddedBranchIDs(branchIDs["Branch 1.1 + Branch 4.1.1"])
			messageMetadata.SetStructureDetails(&markers.StructureDetails{
				PastMarkers:   markers.NewMarkers(),
				FutureMarkers: markers.NewMarkers(),
			})
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

	// // statement 3: "Branch 2"
	{
		message := newTestDataMessagePublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreMessage(message)
		RegisterMessageIDAlias(message.ID(), "Statement3")
		tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetAddedBranchIDs(branchIDs["Branch 2"])
			messageMetadata.SetStructureDetails(&markers.StructureDetails{
				PastMarkers:   markers.NewMarkers(),
				FutureMarkers: markers.NewMarkers(),
			})
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
*/

// TestApprovalWeightManager_updateSequenceSupporters tests the ApprovalWeightManager's functionality regarding sequences.
// The scenario can be found in images/approvalweight-updateSequenceSupporters.png.
func TestApprovalWeightManager_updateSequenceSupporters(t *testing.T) {
	supporters := map[string]*identity.Identity{
		"A": identity.New(ed25519.GenerateKeyPair().PublicKey),
		"B": identity.New(ed25519.GenerateKeyPair().PublicKey),
	}
	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		m := make(map[identity.ID]float64)
		for _, s := range supporters {
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
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewAggregatedBranch(
			ledgerstate.BranchIDs{
				testFramework.BranchIDFromMessage("Message4"): types.Void,
				testFramework.BranchIDFromMessage("Message5"): types.Void,
			}).ID(),
			"Branch4+5",
		)
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
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewAggregatedBranch(
			ledgerstate.BranchIDs{
				testFramework.BranchIDFromMessage("Message4"): types.Void,
				testFramework.BranchIDFromMessage("Message5"): types.Void,
				testFramework.BranchIDFromMessage("Message7"): types.Void,
			}).ID(),
			"Branch4+5+7",
		)
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("E"), WithOutput("I", 500))
		testFramework.IssueMessages("Message8").WaitApprovalWeightProcessed()
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewBranchID(testFramework.TransactionID("Message8")), "Branch8")
		ledgerstate.RegisterBranchIDAlias(ledgerstate.NewAggregatedBranch(
			ledgerstate.BranchIDs{
				testFramework.BranchIDFromMessage("Message4"): types.Void,
				testFramework.BranchIDFromMessage("Message5"): types.Void,
				testFramework.BranchIDFromMessage("Message8"): types.Void,
			}).ID(),
			"Branch4+5+8",
		)
		_, err := tangle.Booker.MessageBranchIDs(testFramework.Message("Message8").ID())
		require.NoError(t, err)
	}
}

func TestOutOfOrderStatments(t *testing.T) {
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
	tangle.Setup()
	testEventMock := NewEventMock(t, tangle.ApprovalWeightManager)
	testFramework := NewMessageTestFramework(tangle, WithGenesisOutput("A", 500), WithGenesisOutput("B", 500))

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.3)

		IssueAndValidateMessageApproval(t, "Message1", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.30,
		})
	}
	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.45)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.15)

		IssueAndValidateMessageApproval(t, "Message2", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.45,
			*markers.NewMarker(1, 2): 0.15,
		})
	}
	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithIssuer(nodes["C"].PublicKey()), WithInputs("A"), WithOutput("A3", 500))
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.70)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.40)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 0.25)

		IssueAndValidateMessageApproval(t, "Message3", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.70,
			*markers.NewMarker(1, 2): 0.40,
			*markers.NewMarker(1, 3): 0.25,
		})
	}
	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.90)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.60)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 0.45)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.20)

		IssueAndValidateMessageApproval(t, "Message4", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.90,
			*markers.NewMarker(1, 2): 0.60,
			*markers.NewMarker(1, 3): 0.45,
			*markers.NewMarker(1, 4): 0.20,
		})
	}
	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A3"), WithOutput("A5", 500))
		testFramework.RegisterBranchID("A", "Message5")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 0.90)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 0.90)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.50)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.30)

		IssueAndValidateMessageApproval(t, "Message5", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 0.90,
			*markers.NewMarker(1, 2): 0.90,
			*markers.NewMarker(1, 3): 0.75,
			*markers.NewMarker(1, 4): 0.50,
			*markers.NewMarker(1, 5): 0.30,
		})
	}
	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message4"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A3"), WithOutput("B6", 500))
		testFramework.RegisterBranchID("B", "Message6")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 0.85)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.60)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 5), 0.10)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("A"), 0.30)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.10)

		IssueAndValidateMessageApproval(t, "Message6", testEventMock, testFramework, map[string]float64{
			"A": 0.3,
			"B": 0.1,
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
		testFramework.CreateMessage("Message7", WithStrongParents("Message3"), WithIssuer(nodes["B"].PublicKey()), WithInputs("B"), WithOutput("B7", 500))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)

		IssueAndValidateMessageApproval(t, "Message7", testEventMock, testFramework, map[string]float64{
			"A": 0.30,
			"B": 0.1,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 0.60,
			*markers.NewMarker(1, 5): 0.30,
			*markers.NewMarker(2, 5): 0.10,
		})
	}
	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message3"), WithIssuer(nodes["D"].PublicKey()), WithInputs("B"), WithOutput("B8", 500))
		testFramework.RegisterBranchID("C", "Message7")
		testFramework.RegisterBranchID("D", "Message8")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(3, 4), 0.20)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("C"), 0.15)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("D"), 0.20)

		IssueAndValidateMessageApproval(t, "Message8", testEventMock, testFramework, map[string]float64{
			"A": 0.30,
			"B": 0.10,
			"C": 0.15,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1,
			*markers.NewMarker(1, 2): 1,
			*markers.NewMarker(1, 3): 1,
			*markers.NewMarker(1, 4): 0.60,
			*markers.NewMarker(1, 5): 0.30,
			*markers.NewMarker(2, 5): 0.10,
			*markers.NewMarker(3, 4): 0.20,
		})
	}
	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message6", "Message7"), WithIssuer(nodes["A"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.60)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 5), 0.40)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(4, 6), 0.30)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("A"), 0.0)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.40)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("C"), 0.45)

		IssueAndValidateMessageApproval(t, "Message9", testEventMock, testFramework, map[string]float64{
			"A": 0,
			"B": 0.40,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1.0,
			*markers.NewMarker(1, 2): 1.0,
			*markers.NewMarker(1, 3): 1.0,
			*markers.NewMarker(1, 4): 0.60,
			*markers.NewMarker(1, 5): 0.30,
			*markers.NewMarker(2, 5): 0.40,
			*markers.NewMarker(3, 4): 0.20,
			*markers.NewMarker(4, 6): 0.30,
		})
	}
	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 5), 0.55)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(4, 6), 0.45)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(4, 7), 0.15)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.55)

		IssueAndValidateMessageApproval(t, "Message10", testEventMock, testFramework, map[string]float64{
			"A": 0,
			"B": 0.55,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1.0,
			*markers.NewMarker(1, 2): 1.0,
			*markers.NewMarker(1, 3): 1.0,
			*markers.NewMarker(1, 4): 0.75,
			*markers.NewMarker(1, 5): 0.30,
			*markers.NewMarker(2, 5): 0.55,
			*markers.NewMarker(3, 4): 0.20,
			*markers.NewMarker(4, 6): 0.45,
			*markers.NewMarker(4, 7): 0.15,
		})
	}
	// ISSUE Message11
	{
		// We skip ahead with the Sequence Number
		testFramework.CreateMessage("Message11", WithStrongParents("Message5"), WithIssuer(nodes["E"].PublicKey()), WithSequenceNumber(1000))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.40)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 6), 0.10)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("A"), 0.10)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("B"), 0.45)

		IssueAndValidateMessageApproval(t, "Message11", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1.0,
			*markers.NewMarker(1, 2): 1.0,
			*markers.NewMarker(1, 3): 1.0,
			*markers.NewMarker(1, 4): 0.75,
			*markers.NewMarker(1, 5): 0.40,
			*markers.NewMarker(2, 5): 0.55,
			*markers.NewMarker(3, 4): 0.20,
			*markers.NewMarker(4, 6): 0.45,
			*markers.NewMarker(4, 7): 0.15,
			*markers.NewMarker(1, 6): 0.10,
		})
	}
	// ISSUE Message12
	{
		// We simulate an "old" vote
		testFramework.CreateMessage("Message12", WithStrongParents("Message10"), WithIssuer(nodes["E"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 5), 0.55)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(4, 6), 0.55)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(4, 7), 0.25)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(4, 8), 0.10)

		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("C"), 0.55)

		IssueAndValidateMessageApproval(t, "Message12", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.55,
			"D": 0.20,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1.0,
			*markers.NewMarker(1, 2): 1.0,
			*markers.NewMarker(1, 3): 1.0,
			*markers.NewMarker(1, 4): 0.75,
			*markers.NewMarker(1, 5): 0.40,
			*markers.NewMarker(2, 5): 0.55,
			*markers.NewMarker(3, 4): 0.20,
			*markers.NewMarker(4, 6): 0.55,
			*markers.NewMarker(4, 7): 0.25,
			*markers.NewMarker(4, 8): 0.10,
		})
	}
	// ISSUE Message12.1
	{
		// We simulate an "old" vote
		testFramework.CreateMessage("Message12.1", WithStrongParents("Message2"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A"), WithOutput("A12.1", 500))
		testFramework.RegisterBranchID("X", "Message3")
		testFramework.RegisterBranchID("Y", "Message12.1")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(5, 3), 0.10)

		// **********
		// **********
		// **********
		// **********
		// **********
		// ALL WRONG
		// **********
		// **********
		// **********
		// **********
		// **********
		// **********
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("X"), 1.0)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("X"), 0.9)
		testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Y"), 0.1)
		//testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("Y"), 0.0)

		IssueAndValidateMessageApproval(t, "Message12.1", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.55,
			"D": 0.20,
			"X": 0.90,
			"Y": 0.10,
		}, map[markers.Marker]float64{
			*markers.NewMarker(1, 1): 1.0,
			*markers.NewMarker(1, 2): 1.0,
			*markers.NewMarker(1, 3): 1.0,
			*markers.NewMarker(1, 4): 0.75,
			*markers.NewMarker(1, 5): 0.40,
			*markers.NewMarker(2, 5): 0.55,
			*markers.NewMarker(3, 4): 0.20,
			*markers.NewMarker(4, 6): 0.55,
			*markers.NewMarker(4, 7): 0.25,
			*markers.NewMarker(4, 8): 0.10,
		})
	}
	/*
		// ISSUE Message13
		{
			// We simulate an "old" vote
			testFramework.CreateMessage("Message13", WithStrongParents("Message6", "Message8"), WithIssuer(nodes["E"].PublicKey()))

			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 4), 0.75)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 5), 0.55)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(3, 4), 0.30)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(5, 6), 0.10)
			// Called again from the 3,4 path
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 1), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 2), 1.0)
			testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 3), 1.0)

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("C"), 0.45)

			// TODO: SHOULD ADD TO D WITHOUT UPDATING X, CAUSE A MORE RECENT VOTE DID IT

			testEventMock.Expect("BranchWeightChanged", testFramework.BranchID("D"), 0.30)

			IssueAndValidateMessageApproval(t, "Message13", testEventMock, testFramework, map[string]float64{
				"A": 0.10,
				"B": 0.45,
				"C": 0.45,
				"D": 0.30,
			}, map[markers.Marker]float64{
				*markers.NewMarker(1, 1): 1.0,
				*markers.NewMarker(1, 2): 1.0,
				*markers.NewMarker(1, 3): 1.0,
				*markers.NewMarker(1, 4): 0.75,
				*markers.NewMarker(1, 5): 0.40,
				*markers.NewMarker(2, 5): 0.55,
				*markers.NewMarker(3, 4): 0.30,
				*markers.NewMarker(4, 6): 0.55,
				*markers.NewMarker(4, 7): 0.25,
				*markers.NewMarker(4, 8): 0.10,
				*markers.NewMarker(5, 6): 0.10,
			})
		}
	*/

	testEventMock.AssertExpectations(t)
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

func validateStatementResults(t *testing.T, approvalWeightManager *ApprovalWeightManager, branchIDs map[string]ledgerstate.BranchID, supporter Voter, expectedResults map[string]bool) {
	for branchIDString, expectedResult := range expectedResults {
		var actualResult bool
		conflictBranchIDs, err := approvalWeightManager.tangle.LedgerState.BranchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchIDs[branchIDString]))
		if err != nil {
			panic(err)
		}
		supporters := approvalWeightManager.SupportersOfConflictBranches(conflictBranchIDs)
		if supporters != nil {
			actualResult = supporters.Has(supporter)
		}

		assert.Equalf(t, expectedResult, actualResult, "%s(%s) does not match", branchIDString, branchIDs[branchIDString])
	}
}
