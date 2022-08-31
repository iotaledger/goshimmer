//nolint:dupl
package tangleold

import (
	"fmt"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/thresholdmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

func BenchmarkApprovalWeightManager_ProcessBlock_Conflicts(b *testing.B) {
	voters := map[string]*identity.Identity{
		"A": identity.New(ed25519.GenerateKeyPair().PublicKey),
		"B": identity.New(ed25519.GenerateKeyPair().PublicKey),
	}
	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		m := make(map[identity.ID]float64)
		ei := epoch.IndexFromTime(time.Now())
		for _, s := range voters {
			weightProvider.Update(ei, s.ID())
			m[s.ID()] = 100
		}
		return m
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

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

func TestConflictWeightMarshalling(t *testing.T) {
	conflictWeight := NewConflictWeight(randomConflictID())
	conflictWeight.SetWeight(5.1234)
	conflictWeightDecoded := new(ConflictWeight)
	err := conflictWeightDecoded.FromBytes(lo.PanicOnErr(conflictWeight.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, lo.PanicOnErr(conflictWeight.Bytes()), lo.PanicOnErr(conflictWeightDecoded.Bytes()))
	assert.Equal(t, conflictWeight.Weight(), conflictWeightDecoded.Weight())
}

func TestConflictVotersMarshalling(t *testing.T) {
	conflictVoters := NewConflictVoters(randomConflictID())

	for i := 0; i < 100; i++ {
		conflictVoters.AddVoter(identity.GenerateIdentity().ID())
	}
	conflictVotersFromBytes := new(ConflictVoters)
	err := conflictVotersFromBytes.FromBytes(lo.PanicOnErr(conflictVoters.Bytes()))
	require.NoError(t, err)

	// verify that conflictVotersFromBytes has all voters from conflictVoters
	assert.Equal(t, conflictVoters.Voters().Set.Size(), conflictVotersFromBytes.Voters().Set.Size())
	conflictVoters.Voters().Set.ForEach(func(voter Voter) {
		assert.True(t, conflictVotersFromBytes.Voters().Set.Has(voter))
	})
}

// TestApprovalWeightManager_updateConflictVoters tests the ApprovalWeightManager's functionality regarding conflictes.
// The scenario can be found in images/approvalweight-updateConflictSupporters.png.
func TestApprovalWeightManager_updateConflictVoters(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		nodeID := identity.NewID(keyPair.PublicKey)
		ei := epoch.IndexFromTime(time.Now())

		weightProvider.Update(ei, nodeID)
		return map[identity.ID]float64{
			nodeID: 100,
		}
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

	tangle := NewTestTangle(ApprovalWeights(weightProvider), WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()
	approvalWeightManager := tangle.ApprovalWeightManager

	resourceIDs := map[string]utxo.OutputID{
		"Conflict 1": randomResourceID(),
		"Conflict 2": randomResourceID(),
		"Conflict 3": randomResourceID(),
		"Conflict 4": randomResourceID(),
		"Conflict 5": randomResourceID(),
	}

	conflictIDs := map[string]*set.AdvancedSet[utxo.TransactionID]{
		"Conflict 1":     set.NewAdvancedSet(randomConflictID()),
		"Conflict 1.1":   set.NewAdvancedSet(randomConflictID()),
		"Conflict 1.2":   set.NewAdvancedSet(randomConflictID()),
		"Conflict 1.3":   set.NewAdvancedSet(randomConflictID()),
		"Conflict 2":     set.NewAdvancedSet(randomConflictID()),
		"Conflict 3":     set.NewAdvancedSet(randomConflictID()),
		"Conflict 4":     set.NewAdvancedSet(randomConflictID()),
		"Conflict 4.1":   set.NewAdvancedSet(randomConflictID()),
		"Conflict 4.1.1": set.NewAdvancedSet(randomConflictID()),
		"Conflict 4.1.2": set.NewAdvancedSet(randomConflictID()),
		"Conflict 4.2":   set.NewAdvancedSet(randomConflictID()),
	}

	createConflict(tangle, "Conflict 1", conflictIDs, set.NewAdvancedSet[utxo.TransactionID](), resourceIDs["Conflict 1"])
	createConflict(tangle, "Conflict 2", conflictIDs, set.NewAdvancedSet[utxo.TransactionID](), resourceIDs["Conflict 1"])
	createConflict(tangle, "Conflict 3", conflictIDs, set.NewAdvancedSet[utxo.TransactionID](), resourceIDs["Conflict 2"])
	createConflict(tangle, "Conflict 4", conflictIDs, set.NewAdvancedSet[utxo.TransactionID](), resourceIDs["Conflict 2"])

	createConflict(tangle, "Conflict 1.1", conflictIDs, conflictIDs["Conflict 1"], resourceIDs["Conflict 3"])
	createConflict(tangle, "Conflict 1.2", conflictIDs, conflictIDs["Conflict 1"], resourceIDs["Conflict 3"])
	createConflict(tangle, "Conflict 1.3", conflictIDs, conflictIDs["Conflict 1"], resourceIDs["Conflict 3"])

	createConflict(tangle, "Conflict 4.1", conflictIDs, conflictIDs["Conflict 4"], resourceIDs["Conflict 4"])
	createConflict(tangle, "Conflict 4.2", conflictIDs, conflictIDs["Conflict 4"], resourceIDs["Conflict 4"])

	createConflict(tangle, "Conflict 4.1.1", conflictIDs, conflictIDs["Conflict 4.1"], resourceIDs["Conflict 5"])
	createConflict(tangle, "Conflict 4.1.2", conflictIDs, conflictIDs["Conflict 4.1"], resourceIDs["Conflict 5"])

	conflictIDs["Conflict 1.1 + Conflict 4.1.1"] = set.NewAdvancedSet[utxo.TransactionID]()
	conflictIDs["Conflict 1.1 + Conflict 4.1.1"].AddAll(conflictIDs["Conflict 1.1"])
	conflictIDs["Conflict 1.1 + Conflict 4.1.1"].AddAll(conflictIDs["Conflict 4.1.1"])

	// Issue statements in different order to make sure that no information is lost when nodes apply statements in arbitrary order

	block1 := newTestDataBlockPublicKey("test1", keyPair.PublicKey)
	block2 := newTestDataBlockPublicKey("test2", keyPair.PublicKey)
	// statement 2: "Conflict 4.1.2"
	{
		block := block2
		tangle.Storage.StoreBlock(block)
		block.ID().RegisterAlias("Statement2")
		tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
			blockMetadata.SetAddedConflictIDs(conflictIDs["Conflict 4.1.2"])
			blockMetadata.SetStructureDetails(markers.NewStructureDetails())
		})
		approvalWeightManager.updateConflictVoters(block)

		expectedResults := map[string]bool{
			"Conflict 1":     false,
			"Conflict 1.1":   false,
			"Conflict 1.2":   false,
			"Conflict 1.3":   false,
			"Conflict 2":     false,
			"Conflict 3":     false,
			"Conflict 4":     true,
			"Conflict 4.1":   true,
			"Conflict 4.1.1": false,
			"Conflict 4.1.2": true,
			"Conflict 4.2":   false,
		}
		validateStatementResults(t, approvalWeightManager, conflictIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}

	// statement 1: "Conflict 1.1 + Conflict 4.1.1"
	{
		block := block1
		tangle.Storage.StoreBlock(block)
		block.ID().RegisterAlias("Statement1")
		tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
			blockMetadata.SetAddedConflictIDs(conflictIDs["Conflict 1.1 + Conflict 4.1.1"])
			blockMetadata.SetStructureDetails(markers.NewStructureDetails())
		})
		approvalWeightManager.updateConflictVoters(block)

		expectedResults := map[string]bool{
			"Conflict 1":     true,
			"Conflict 1.1":   true,
			"Conflict 1.2":   false,
			"Conflict 1.3":   false,
			"Conflict 2":     false,
			"Conflict 3":     false,
			"Conflict 4":     true,
			"Conflict 4.1":   true,
			"Conflict 4.1.1": false,
			"Conflict 4.1.2": true,
			"Conflict 4.2":   false,
		}
		validateStatementResults(t, approvalWeightManager, conflictIDs, identity.NewID(keyPair.PublicKey), expectedResults)
	}

	// statement 3: "Conflict 2"
	{
		block := newTestDataBlockPublicKey("test", keyPair.PublicKey)
		tangle.Storage.StoreBlock(block)
		block.ID().RegisterAlias("Statement3")
		tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
			blockMetadata.SetAddedConflictIDs(conflictIDs["Conflict 2"])
			blockMetadata.SetStructureDetails(markers.NewStructureDetails())
		})
		approvalWeightManager.updateConflictVoters(block)

		expectedResults := map[string]bool{
			"Conflict 1":     false,
			"Conflict 1.1":   false,
			"Conflict 1.2":   false,
			"Conflict 1.3":   false,
			"Conflict 2":     true,
			"Conflict 3":     false,
			"Conflict 4":     true,
			"Conflict 4.1":   true,
			"Conflict 4.1.1": false,
			"Conflict 4.1.2": true,
			"Conflict 4.2":   false,
		}
		validateStatementResults(t, approvalWeightManager, conflictIDs, identity.NewID(keyPair.PublicKey), expectedResults)
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
			ei := epoch.IndexFromTime(time.Now())

			weightProvider.Update(ei, s.ID())
			m[s.ID()] = 100
		}
		return m
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

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

		markersMap["0,1"].SetPastMarkerGap(50)
		markersMap["1,2"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,1"]}, increaseIndexCallback)
		markersMap["1,3"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,2"]}, increaseIndexCallback)
		markersMap["1,4"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,3"]}, increaseIndexCallback)
		markersMap["1,5"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["1,4"]}, increaseIndexCallback)

		markersMap["0,3"].SetPastMarkerGap(50)
		markersMap["1,4"].SetPastMarkerGap(50)
		markersMap["2,5"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,3"], markersMap["1,4"]}, increaseIndexCallback)
		markersMap["2,6"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["0,4"], markersMap["2,5"]}, increaseIndexCallback)
		markersMap["2,7"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,6"]}, increaseIndexCallback)
		markersMap["2,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,7"]}, increaseIndexCallback)

		markersMap["2,7"].SetPastMarkerGap(50)
		markersMap["3,8"], _ = tangle.Booker.MarkersManager.Manager.InheritStructureDetails([]*markers.StructureDetails{markersMap["2,7"]}, increaseIndexCallback)
		markersMap["1,4"].SetPastMarkerGap(50)
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

// TestApprovalWeightManager_ProcessBlock tests the whole functionality of the ApprovalWeightManager.
// The scenario can be found in images/approvalweight-processBlock.png.
func TestApprovalWeightManager_ProcessBlock(t *testing.T) {
	processBlkScenario := ProcessBlockScenario(t)
	defer func(processBlkScenario *TestScenario, t *testing.T) {
		if err := processBlkScenario.Cleanup(t); err != nil {
			require.NoError(t, err)
		}
	}(processBlkScenario, t)

	for processBlkScenario.HasNext() {
		processBlkScenario.Next(nil)
	}
}

func TestAggregatedConflictApproval(t *testing.T) {
	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			ei := epoch.IndexFromTime(time.Now())
			weightProvider.Update(ei, node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

	tangle := NewTestTangle(ApprovalWeights(weightProvider))
	defer tangle.Shutdown()
	tangle.Setup()

	testFramework := NewBlockTestFramework(tangle, WithGenesisOutput("G1", 500), WithGenesisOutput("G2", 500))

	// ISSUE Block1
	{
		testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()), WithInputs("G1"), WithOutput("A", 500))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block1").RegisterAlias("Conflict1")
	}

	// ISSUE Block2
	{
		testFramework.CreateBlock("Block2", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()), WithInputs("G2"), WithOutput("B", 500))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block2").RegisterAlias("Conflict2")
	}

	// ISSUE Block3
	{
		testFramework.CreateBlock("Block3", WithStrongParents("Block2"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("C", 500))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block3").RegisterAlias("Conflict3")
	}

	// ISSUE Block4
	{
		testFramework.CreateBlock("Block4", WithStrongParents("Block2"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block4").RegisterAlias("Conflict4")
	}

	// ISSUE Block5
	{
		testFramework.CreateBlock("Block5", WithStrongParents("Block4", "Block1"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("E", 500))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block5").RegisterAlias("Conflict5")
	}

	// ISSUE Block6
	{
		testFramework.CreateBlock("Block6", WithStrongParents("Block4", "Block1"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A"), WithOutput("F", 500))
		testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block6").RegisterAlias("Conflict6")

		_, err := tangle.Booker.BlockConflictIDs(testFramework.Block("Block6").ID())
		require.NoError(t, err)
	}

	// ISSUE Block7
	{
		testFramework.CreateBlock("Block7", WithStrongParents("Block5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("E"), WithOutput("H", 500))
		testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block7").RegisterAlias("Conflict7")
	}

	// ISSUE Block8
	{
		testFramework.CreateBlock("Block8", WithStrongParents("Block5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("E"), WithOutput("I", 500))
		testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()
		testFramework.TransactionID("Block8").RegisterAlias("Conflict8")
		_, err := tangle.Booker.BlockConflictIDs(testFramework.Block("Block8").ID())
		require.NoError(t, err)
	}
}

func TestOutOfOrderStatements(t *testing.T) {
	debug.SetEnabled(true)

	nodes := make(map[string]*identity.Identity)
	for _, node := range []string{"A", "B", "C", "D", "E"} {
		nodes[node] = identity.GenerateIdentity()
		identity.RegisterIDAlias(nodes[node].ID(), node)
	}

	var weightProvider *CManaWeightProvider
	manaRetrieverMock := func() map[identity.ID]float64 {
		for _, node := range nodes {
			ei := epoch.IndexFromTime(time.Now())
			weightProvider.Update(ei, node.ID())
		}
		return map[identity.ID]float64{
			nodes["A"].ID(): 30,
			nodes["B"].ID(): 15,
			nodes["C"].ID(): 25,
			nodes["D"].ID(): 20,
			nodes["E"].ID(): 10,
		}
	}
	confirmedRetrieverFunc := func() epoch.Index { return 0 }
	weightProvider = NewCManaWeightProvider(manaRetrieverMock, time.Now, confirmedRetrieverFunc)

	tangle := NewTestTangle(ApprovalWeights(weightProvider), WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	tangle.Booker.MarkersManager.Options.MaxPastMarkerDistance = 3

	tangle.Setup()
	testEventMock := NewEventMock(t, tangle.ApprovalWeightManager)
	testFramework := NewBlockTestFramework(tangle, WithGenesisOutput("A", 500), WithGenesisOutput("B", 500))

	// ISSUE Block1
	{
		testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithIssuer(nodes["A"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.3)

		IssueAndValidateBlockApproval(t, "Block1", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 0.30,
		})
	}
	// ISSUE Block2
	{
		testFramework.CreateBlock("Block2", WithStrongParents("Block1"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.45)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.15)

		IssueAndValidateBlockApproval(t, "Block2", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 0.45,
			markers.NewMarker(0, 2): 0.15,
		})
	}
	// ISSUE Block3
	{
		testFramework.CreateBlock("Block3", WithStrongParents("Block2"), WithIssuer(nodes["C"].PublicKey()), WithInputs("A"), WithOutput("A3", 500))
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.70)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.40)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.25)

		IssueAndValidateBlockApproval(t, "Block3", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 0.70,
			markers.NewMarker(0, 2): 0.40,
			markers.NewMarker(0, 3): 0.25,
		})
	}
	// ISSUE Block4
	{
		testFramework.CreateBlock("Block4", WithStrongParents("Block3"), WithIssuer(nodes["D"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 0.90)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.60)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.45)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.20)

		IssueAndValidateBlockApproval(t, "Block4", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 0.90,
			markers.NewMarker(0, 2): 0.60,
			markers.NewMarker(0, 3): 0.45,
			markers.NewMarker(0, 4): 0.20,
		})
	}
	// ISSUE Block5
	{
		testFramework.CreateBlock("Block5", WithStrongParents("Block4"), WithIssuer(nodes["A"].PublicKey()), WithInputs("A3"), WithOutput("A5", 500))
		testFramework.RegisterConflictID("A", "Block5")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 0.90)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.50)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.30)

		IssueAndValidateBlockApproval(t, "Block5", testEventMock, testFramework, map[string]float64{}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 0.90,
			markers.NewMarker(0, 2): 0.90,
			markers.NewMarker(0, 3): 0.75,
			markers.NewMarker(0, 4): 0.50,
			markers.NewMarker(0, 5): 0.30,
		})
	}

	// ISSUE Block6
	{
		testFramework.CreateBlock("Block6", WithStrongParents("Block4"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A3"), WithOutput("B6", 500))
		testFramework.RegisterConflictID("B", "Block6")

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 1), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 2), 1.0)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 0.85)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.60)

		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("A"), 0.30)
		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("B"), 0.10)

		IssueAndValidateBlockApproval(t, "Block6", testEventMock, testFramework, map[string]float64{
			"A": 0.3,
			"B": 0.1,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 0.85,
			markers.NewMarker(0, 4): 0.60,
			markers.NewMarker(0, 5): 0.30,
		})
	}

	// ISSUE Block7
	{
		testFramework.CreateBlock("Block7", WithStrongParents("Block3"), WithIssuer(nodes["B"].PublicKey()), WithInputs("B"), WithOutput("B7", 500))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 1.0)

		IssueAndValidateBlockApproval(t, "Block7", testEventMock, testFramework, map[string]float64{
			"A": 0.30,
			"B": 0.1,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 1,
			markers.NewMarker(0, 4): 0.60,
			markers.NewMarker(0, 5): 0.30,
		})
	}
	// ISSUE Block8
	{
		testFramework.CreateBlock("Block8", WithStrongParents("Block3"), WithIssuer(nodes["D"].PublicKey()), WithInputs("B"), WithOutput("B8", 500))
		testFramework.RegisterConflictID("C", "Block7")
		testFramework.RegisterConflictID("D", "Block8")

		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("C"), 0.15)
		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("D"), 0.20)

		IssueAndValidateBlockApproval(t, "Block8", testEventMock, testFramework, map[string]float64{
			"A": 0.30,
			"B": 0.10,
			"C": 0.15,
			"D": 0.20,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 1,
			markers.NewMarker(0, 4): 0.60,
			markers.NewMarker(0, 5): 0.30,
		})
	}

	// ISSUE Block9
	{
		testFramework.CreateBlock("Block9", WithStrongParents("Block6", "Block7"), WithIssuer(nodes["A"].PublicKey()))

		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("A"), 0.0)
		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("B"), 0.40)
		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("C"), 0.45)

		IssueAndValidateBlockApproval(t, "Block9", testEventMock, testFramework, map[string]float64{
			"A": 0,
			"B": 0.40,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 1,
			markers.NewMarker(0, 4): 0.60,
			markers.NewMarker(0, 5): 0.30,
		})
	}
	// ISSUE Block10
	{
		testFramework.CreateBlock("Block10", WithStrongParents("Block9"), WithIssuer(nodes["B"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 0.75)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.15)

		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("B"), 0.55)

		IssueAndValidateBlockApproval(t, "Block10", testEventMock, testFramework, map[string]float64{
			"A": 0,
			"B": 0.55,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 1,
			markers.NewMarker(0, 4): 0.75,
			markers.NewMarker(0, 5): 0.30,
			markers.NewMarker(1, 5): 0.15,
		})
	}

	// ISSUE Block11
	{
		// We skip ahead with the Sequence Number
		testFramework.CreateBlock("Block11", WithStrongParents("Block5"), WithIssuer(nodes["E"].PublicKey()), WithSequenceNumber(10000000000))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.40)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 6), 0.10)

		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("A"), 0.10)
		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("B"), 0.45)

		IssueAndValidateBlockApproval(t, "Block11", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.45,
			"D": 0.20,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 1,
			markers.NewMarker(0, 4): 0.75,
			markers.NewMarker(0, 5): 0.40,
			markers.NewMarker(0, 6): 0.10,
			markers.NewMarker(1, 5): 0.15,
		})
	}

	// ISSUE Block12
	{
		// We simulate an "old" vote
		testFramework.CreateBlock("Block12", WithStrongParents("Block10"), WithIssuer(nodes["E"].PublicKey()))

		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.25)
		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 6), 0.10)

		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("C"), 0.55)

		IssueAndValidateBlockApproval(t, "Block12", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.55,
			"D": 0.20,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 1,
			markers.NewMarker(0, 4): 0.75,
			markers.NewMarker(0, 5): 0.40,
			markers.NewMarker(0, 6): 0.10,
			markers.NewMarker(1, 5): 0.25,
			markers.NewMarker(1, 6): 0.10,
		})
	}

	testEventMock.AssertExpectations(t)

	// ISSUE Block13
	{
		// We simulate an "old" vote
		testFramework.CreateBlock("Block13", WithStrongParents("Block2"), WithIssuer(nodes["E"].PublicKey()), WithInputs("A"), WithOutput("A13", 500))
		testFramework.RegisterConflictID("X", "Block3")
		testFramework.RegisterConflictID("Y", "Block13")

		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("X"), 1.0)

		IssueAndValidateBlockApproval(t, "Block13", testEventMock, testFramework, map[string]float64{
			"A": 0.10,
			"B": 0.45,
			"C": 0.55,
			"D": 0.20,
			"X": 1.00,
			"Y": 0.00,
		}, map[markers.Marker]float64{
			markers.NewMarker(0, 1): 1,
			markers.NewMarker(0, 2): 1,
			markers.NewMarker(0, 3): 1,
			markers.NewMarker(0, 4): 0.75,
			markers.NewMarker(0, 5): 0.40,
			markers.NewMarker(0, 6): 0.10,
			markers.NewMarker(1, 5): 0.25,
			markers.NewMarker(1, 6): 0.10,
		})
	}
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
	votes.M.ForEach(func(node *thresholdmap.Element[markers.Index, uint64]) bool {
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
		assert.Equal(t, markerAlias, fmt.Sprintf("%d,%d", markersMap[markerAlias].PastMarkers().Marker().SequenceID(), markersMap[markerAlias].PastMarkers().Marker().Index()))

		voters := approvalWeightManager.markerVotes(markersMap[markerAlias].PastMarkers().Marker())

		assert.Equal(t, len(expectedVotersOfMarker), len(voters), "size of voters for Marker("+markerAlias+") does not match")
		for _, voter := range expectedVotersOfMarker {
			_, voterExists := voters[voter.ID()]
			assert.True(t, voterExists)
		}
	}
}

func approveMarkers(approvalWeightManager *ApprovalWeightManager, voter *identity.Identity, markersToApprove ...markers.Marker) (block *Block) {
	block = newTestDataBlockPublicKey("test", voter.PublicKey())
	approvalWeightManager.tangle.Storage.StoreBlock(block)
	approvalWeightManager.tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
		newStructureDetails := markers.NewStructureDetails()
		newStructureDetails.SetIsPastMarker(true)
		newStructureDetails.SetPastMarkers(markers.NewMarkers(markersToApprove...))

		blockMetadata.SetStructureDetails(newStructureDetails)
	})

	return
}

func increaseIndexCallback(markers.SequenceID, markers.Index) bool {
	return true
}

func getSingleConflict(conflictes map[string]*set.AdvancedSet[utxo.TransactionID], alias string) utxo.TransactionID {
	if conflictes[alias].Size() != 1 {
		panic(fmt.Sprintf("Conflictes with alias %s are multiple conflictes, not a single one: %+v", alias, conflictes[alias]))
	}

	for it := conflictes[alias].Iterator(); it.HasNext(); {
		return it.Next()
	}

	return utxo.EmptyTransactionID
}

func createConflict(tangle *Tangle, conflictAlias string, conflictIDs map[string]*set.AdvancedSet[utxo.TransactionID], parentConflictIDs *set.AdvancedSet[utxo.TransactionID], conflictID utxo.OutputID) {
	conflict := getSingleConflict(conflictIDs, conflictAlias)
	tangle.Ledger.ConflictDAG.CreateConflict(conflict, parentConflictIDs, set.NewAdvancedSet(conflictID))
	conflict.RegisterAlias(conflictAlias)
}

func validateStatementResults(t *testing.T, approvalWeightManager *ApprovalWeightManager, conflictIDs map[string]utxo.TransactionIDs, voter Voter, expectedResults map[string]bool) {
	for conflictIDString, expectedResult := range expectedResults {
		var actualResult bool
		for it := conflictIDs[conflictIDString].Iterator(); it.HasNext(); {
			voters := approvalWeightManager.VotersOfConflict(it.Next())
			if voters != nil {
				actualResult = voters.Set.Has(voter)
			}
			if !actualResult {
				break
			}
		}

		assert.Equalf(t, expectedResult, actualResult, "%s(%s) does not match", conflictIDString, conflictIDs[conflictIDString])
	}
}
