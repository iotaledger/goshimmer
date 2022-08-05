package tangleold

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/markers"
)

func TestTipManager_DataBlockTips(t *testing.T) {
	tangle := NewTestTangle()
	defer func(tangle *Tangle) {
		_ = tangle.Prune()
		tangle.Shutdown()
	}(tangle)
	tangle.Setup()
	tipManager := tangle.TipManager

	// set up scenario (images/tipmanager-DataBlockTips-test.png)
	blocks := make(map[string]*Block)

	// without any tip -> genesis
	{
		parents := tipManager.Tips(nil, 2)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, EmptyBlockID)
	}
	fmt.Println("send genesis")
	// without any count -> 1 tip, in this case genesis
	{
		parents := tipManager.Tips(nil, 0)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, EmptyBlockID)
	}
	fmt.Println("send blk1")
	// Block 1
	{
		blocks["1"] = createAndStoreParentsDataBlockInMasterConflict(tangle, NewBlockIDs(EmptyBlockID), NewBlockIDs())
		tipManager.AddTip(blocks["1"])
		tangle.TimeManager.updateTime(blocks["1"])
		tangle.TimeManager.updateSyncedState()

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), blocks["1"].ID())

		parents := tipManager.Tips(nil, 2)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, blocks["1"].ID())
	}
	fmt.Println("send blk2")
	// Block 2
	{
		blocks["2"] = createAndStoreParentsDataBlockInMasterConflict(tangle, NewBlockIDs(EmptyBlockID), NewBlockIDs())
		tipManager.AddTip(blocks["2"])

		assert.Equal(t, 2, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), blocks["1"].ID(), blocks["2"].ID())

		parents := tipManager.Tips(nil, 3)
		assert.Len(t, parents, 2)
		assert.Contains(t, parents, blocks["1"].ID(), blocks["2"].ID())
	}
	fmt.Println("send blk3")
	// Block 3
	{
		blocks["3"] = createAndStoreParentsDataBlockInMasterConflict(tangle, NewBlockIDs(blocks["1"].ID(), blocks["2"].ID()), NewBlockIDs())
		tipManager.AddTip(blocks["3"])

		assert.Equal(t, 1, tipManager.TipCount())
		assert.Contains(t, tipManager.tips.Keys(), blocks["3"].ID())

		parents := tipManager.Tips(nil, 2)
		assert.Len(t, parents, 1)
		assert.Contains(t, parents, blocks["3"].ID())
	}
	fmt.Println("send blk3")
	// Add Block 4-8
	{
		tips := NewBlockIDs()
		tips.Add(blocks["3"].ID())
		for count, n := range []int{4, 5, 6, 7, 8} {
			nString := strconv.Itoa(n)
			blocks[nString] = createAndStoreParentsDataBlockInMasterConflict(tangle, NewBlockIDs(blocks["1"].ID()), NewBlockIDs())
			tipManager.AddTip(blocks[nString])
			tips.Add(blocks[nString].ID())

			assert.Equalf(t, count+2, tipManager.TipCount(), "TipCount does not match after adding Block %d", n)
			assert.ElementsMatchf(t, tipManager.tips.Keys(), tips.Slice(), "Elements in strongTips do not match after adding Block %d", n)
			assert.Contains(t, tipManager.tips.Keys(), blocks["3"].ID())
			fmt.Println("send blk", n)
		}
	}

	// now we have 6 tips
	// Tips(4) -> 4
	{
		parents := tipManager.Tips(nil, 4)
		assert.Len(t, parents, 4)
	}
	fmt.Println("select tip1")
	// Tips(8) -> 6
	{
		parents := tipManager.Tips(nil, 8)
		assert.Len(t, parents, 6)
	}
	fmt.Println("select tip2")
	// Tips(0) -> 1
	{
		parents := tipManager.Tips(nil, 0)
		assert.Len(t, parents, 1)
	}
	fmt.Println("select tip3")
}

// TODO: FIX
// func TestTipManager_TransactionTips(t *testing.T) {
// 	// set up scenario (images/tipmanager-TransactionTips-test.png)
// 	tangle := NewTestTangle()
// 	defer tangle.Shutdown()
// 	tipManager := tangle.TipManager
// 	confirmedBlockIDs := NewBlockIDs()
// 	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedBlockIDs: confirmedBlockIDs, confirmedMarkers: markers.NewMarkers(markers.NewMarker(0, 1))}
//
// 	testFramework := NewBlockTestFramework(tangle, WithGenesisOutput("G1", 5), WithGenesisOutput("G2", 8))
//
// 	// region prepare scenario /////////////////////////////////////////////////////////////////////////////////////////
//
// 	// Block 1
// 	{
// 		issueTime := time.Now().Add(-maxParentsTimeDifference - 5*time.Minute)
//
// 		testFramework.CreateBlock(
// 			"Block1",
// 			WithStrongParents("Genesis"),
// 			WithIssuingTime(issueTime),
// 			WithInputs("G1"),
// 			WithOutput("A", 3),
// 			WithOutput("B", 1),
// 			WithOutput("c", 1),
// 		)
// 		testFramework.IssueBlocks("Block1")
// 		// bookBlock(t, tangle, testFramework.Block("Block1"))
// 		testFramework.WaitUntilAllTasksProcessed()
//
// 		tipManager.AddTip(testFramework.Block("Block1"))
// 		assert.Equal(t, 0, tipManager.TipCount())
//
// 		// mark this block as confirmed
// 		confirmedBlockIDs.Add(testFramework.Block("Block1").ID())
// 	}
//
// 	// Block 2
// 	{
// 		testFramework.CreateBlock(
// 			"Block2",
// 			WithStrongParents("Genesis"),
// 			WithInputs("G2"),
// 			WithOutput("D", 6),
// 			WithOutput("E", 1),
// 			WithOutput("F", 1),
// 		)
// 		testFramework.IssueBlocks("Block2")
// 		testFramework.WaitUntilAllTasksProcessed()
//
// 		tipManager.AddTip(testFramework.Block("Block2"))
// 		assert.Equal(t, 1, tipManager.TipCount())
//
// 		// use this block to set TangleTime
// 		tangle.TimeManager.updateTime(testFramework.Block("Block2"))
// 	}
//
// 	// Block 3
// 	{
// 		testFramework.CreateBlock(
// 			"Block3",
// 			WithStrongParents("Genesis"),
// 			WithInputs("A"),
// 			WithOutput("H", 1),
// 			WithOutput("I", 1),
// 			WithOutput("J", 1),
// 		)
// 		testFramework.IssueBlocks("Block3")
// 		testFramework.WaitUntilAllTasksProcessed()
// 		tipManager.AddTip(testFramework.Block("Block3"))
// 		assert.Equal(t, 2, tipManager.TipCount())
// 	}
//
// 	// Block 4
// 	{
//
// 		testFramework.CreateBlock(
// 			"Block4",
// 			WithStrongParents("Genesis", "Block2"),
// 			WithInputs("D"),
// 			WithOutput("K", 1),
// 			WithOutput("L", 1),
// 			WithOutput("M", 1),
// 			WithOutput("N", 1),
// 			WithOutput("O", 1),
// 			WithOutput("P", 1),
// 		)
// 		testFramework.IssueBlocks("Block4")
// 		testFramework.WaitUntilAllTasksProcessed()
// 		tipManager.AddTip(testFramework.Block("Block4"))
// 		assert.Equal(t, 2, tipManager.TipCount())
// 	}
//
// 	// Block 5
// 	{
// 		testFramework.CreateBlock(
// 			"Block5",
// 			WithStrongParents("Genesis", "Block1"),
// 		)
// 		testFramework.IssueBlocks("Block5")
// 		testFramework.WaitUntilAllTasksProcessed()
// 		tipManager.AddTip(testFramework.Block("Block5"))
// 		assert.Equal(t, 3, tipManager.TipCount())
// 	}
//
// 	createScenarioBlockWith1Input1Output := func(blockStringID, inputStringID, outputStringID string, strongParents []string) {
// 		testFramework.CreateBlock(
// 			blockStringID,
// 			WithInputs(inputStringID),
// 			WithOutput(outputStringID, 1),
// 			WithStrongParents(strongParents...),
// 		)
// 		testFramework.IssueBlocks(blockStringID)
// 		testFramework.WaitUntilAllTasksProcessed()
// 	}
//
// 	// Block 6
// 	{
// 		createScenarioBlockWith1Input1Output("Block6", "H", "Q", []string{"Block3", "Block5"})
// 		tipManager.AddTip(testFramework.Block("Block6"))
// 		assert.Equal(t, 2, tipManager.TipCount())
// 	}
//
// 	// Block 7
// 	{
// 		createScenarioBlockWith1Input1Output("Block7", "I", "R", []string{"Block3", "Block5"})
// 		tipManager.AddTip(testFramework.Block("Block7"))
// 		assert.Equal(t, 3, tipManager.TipCount())
// 	}
//
// 	// Block 8
// 	{
// 		createScenarioBlockWith1Input1Output("Block8", "J", "S", []string{"Block3", "Block5"})
// 		tipManager.AddTip(testFramework.Block("Block8"))
// 		assert.Equal(t, 4, tipManager.TipCount())
// 	}
//
// 	// Block 9
// 	{
// 		createScenarioBlockWith1Input1Output("Block9", "K", "T", []string{"Block4"})
// 		tipManager.AddTip(testFramework.Block("Block9"))
// 		assert.Equal(t, 4, tipManager.TipCount())
// 	}
//
// 	// Block 10
// 	{
// 		createScenarioBlockWith1Input1Output("Block10", "L", "U", []string{"Block2", "Block4"})
// 		tipManager.AddTip(testFramework.Block("Block10"))
// 		assert.Equal(t, 5, tipManager.TipCount())
// 	}
//
// 	// Block 11
// 	{
// 		createScenarioBlockWith1Input1Output("Block11", "M", "V", []string{"Block2", "Block4"})
// 		tipManager.AddTip(testFramework.Block("Block11"))
// 		assert.Equal(t, 6, tipManager.TipCount())
// 	}
//
// 	// Block 12
// 	{
// 		createScenarioBlockWith1Input1Output("Block12", "N", "X", []string{"Block3", "Block4"})
// 		tipManager.AddTip(testFramework.Block("Block12"))
// 		assert.Equal(t, 7, tipManager.TipCount())
// 	}
//
// 	// Block 13
// 	{
// 		createScenarioBlockWith1Input1Output("Block13", "O", "Y", []string{"Block4"})
// 		tipManager.AddTip(testFramework.Block("Block13"))
// 		assert.Equal(t, 8, tipManager.TipCount())
// 	}
//
// 	// Block 14
// 	{
// 		createScenarioBlockWith1Input1Output("Block14", "P", "Z", []string{"Block4"})
// 		tipManager.AddTip(testFramework.Block("Block14"))
// 		assert.Equal(t, 9, tipManager.TipCount())
// 	}
//
// 	// Block 15
// 	{
// 		testFramework.CreateBlock(
// 			"Block15",
// 			WithStrongParents("Block10", "Block11"),
// 		)
// 		testFramework.IssueBlocks("Block15")
// 		testFramework.WaitUntilAllTasksProcessed()
// 		tipManager.AddTip(testFramework.Block("Block15"))
// 		assert.Equal(t, 8, tipManager.TipCount())
// 	}
//
// 	// Block 16
// 	{
// 		testFramework.CreateBlock(
// 			"Block16",
// 			WithStrongParents("Block10", "Block11", "Block14"),
// 		)
// 		testFramework.IssueBlocks("Block16")
// 		testFramework.WaitUntilAllTasksProcessed()
// 		tipManager.AddTip(testFramework.Block("Block16"))
// 		assert.Equal(t, 8, tipManager.TipCount())
// 	}
// 	// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////
//
// 	// Block 17
// 	{
// 		testFramework.CreateBlock(
// 			"Block17",
// 			WithStrongParents("Block16"),
// 			WithOutput("OUT17", 8),
// 			WithInputs("Q", "R", "S", "T", "U", "V", "X", "Y"),
// 		)
// 		testFramework.IssueBlocks("Block17").WaitUntilAllTasksProcessed()
// 		parents, err := tipManager.Tips(testFramework.Block("Block17").Payload(), 4)
// 		assert.NoError(t, err)
// 		assert.Equal(t, parents, NewBlockIDs(
// 			testFramework.Block("Block6").ID(),
// 			testFramework.Block("Block7").ID(),
// 			testFramework.Block("Block8").ID(),
// 			testFramework.Block("Block9").ID(),
// 			testFramework.Block("Block10").ID(),
// 			testFramework.Block("Block11").ID(),
// 			testFramework.Block("Block12").ID(),
// 			testFramework.Block("Block13").ID(),
// 		), "Block17 should have the correct parents")
// 	}
//
// 	// Block 18
// 	{
// 		testFramework.CreateBlock(
// 			"Block18",
// 			WithStrongParents("Block16"),
// 			WithOutput("OUT18", 6),
// 			WithInputs("Q", "R", "S", "T", "U", "V"),
// 		)
//
// 		parents, err := tipManager.Tips(testFramework.Block("Block18").Payload(), 4)
// 		fmt.Println(parents, err)
//
// 		assert.NoError(t, err)
// 		// there are possible parents to be selected, however, since the directly referenced blocks are tips as well
// 		// there is a chance that these are doubly selected, resulting 6 to 8 parents
// 		assert.GreaterOrEqual(t, len(parents), 6)
// 		assert.LessOrEqual(t, len(parents), 8)
// 		assert.Contains(t, parents,
// 			testFramework.Block("Block6").ID(),
// 			testFramework.Block("Block7").ID(),
// 			testFramework.Block("Block8").ID(),
// 			testFramework.Block("Block9").ID(),
// 			testFramework.Block("Block10").ID(),
// 			testFramework.Block("Block11").ID(),
// 		)
// 	}
//
// 	// Block 19
// 	{
// 		testFramework.CreateBlock(
// 			"Block19",
// 			WithStrongParents("Block16"),
// 			WithOutput("OUT19", 3),
// 			WithInputs("B", "V", "Z"),
// 		)
//
// 		parents, err := tipManager.Tips(testFramework.Block("Block19").Payload(), 4)
// 		assert.NoError(t, err)
//
// 		// we reference 11, 14 directly. 1 is too old and should not be directly referenced
// 		assert.GreaterOrEqual(t, len(parents), 4)
// 		assert.LessOrEqual(t, len(parents), 8)
// 		assert.Contains(t, parents,
// 			testFramework.Block("Block11").ID(),
// 			testFramework.Block("Block14").ID(),
// 		)
// 		assert.NotContains(t, parents,
// 			testFramework.Block("Block1").ID(),
// 		)
// 	}
//
// 	// Block 20
// 	{
//
// 		testFramework.CreateBlock(
// 			"Block20",
// 			WithStrongParents("Block16"),
// 			WithOutput("OUT20", 9),
// 			WithInputs("Q", "R", "S", "T", "U", "V", "X", "Y", "Z"),
// 		)
//
// 		parents, err := tipManager.Tips(testFramework.Block("Block20").Payload(), 4)
// 		assert.NoError(t, err)
//
// 		// there are 9 inputs to be directly referenced -> we need to reference them via tips (8 tips available)
// 		// due to the tips' nature they contain all transactions in the past cone
// 		assert.Len(t, parents, 8)
// 	}
// }

// Test based on packages/tangle/images/TSC_test_scenario.png except nothing is confirmed.
func TestTipManager_TimeSinceConfirmation_Unconfirmed(t *testing.T) {
	t.Skip("Skip this test.")
	tangle := NewTestTangle()
	tangle.Booker.MarkersManager.Manager = markers.NewManager(markers.WithCacheTime(0), markers.WithMaxPastMarkerDistance(10))
	defer tangle.Shutdown()

	tipManager := tangle.TipManager

	testFramework := NewBlockTestFramework(
		tangle,
	)

	tangle.Setup()

	createTestTangleTSC(t, testFramework)
	var confirmedBlockIDsString []string
	confirmedBlockIDs := prepareConfirmedBlockIDs(testFramework, confirmedBlockIDsString)
	confirmedMarkers := markers.NewMarkers()

	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedBlockIDs: confirmedBlockIDs, confirmedMarkers: confirmedMarkers}
	tangle.TimeManager.updateTime(testFramework.Block("Marker-2/3"))
	tangle.TimeManager.updateSyncedState()

	// Even without any confirmations, it should be possible to attach to genesis.
	assert.True(t, tipManager.isPastConeTimestampCorrect(EmptyBlockID))

	// case 0 - only one block can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/0_2").ID()))
	// case #1
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/3_4").ID()))
	// case #2
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("1/3_4").ID()))
	// case #3
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3_4").ID()))
	// case #4 (marker block)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-1/2").ID()))
	// case #5
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/5_4").ID()))
	// case #6 (attach to unconfirmed block older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSC_2").ID()))
	// case #7
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("3/2_4").ID()))
	// case #8
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3+0/4_3").ID()))
	// case #9
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-4/5").ID()))
	// case #10 (attach to confirmed block older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq2_2").ID()))
	// case #11
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("5/2_4").ID()))
	// case #12 (attach to 0/1-postTSCSeq3_4)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSCSeq3_4").ID()))
	// case #13 (attach to unconfirmed block younger than TSC, with confirmed past marker older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC_2").ID()))
	// case #14
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("6/2_4").ID()))
	// case #15
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq5_4").ID()))
	// case #16
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC-direct_0").ID()))
}

// Test based on packages/tangle/images/TSC_test_scenario.png.
func TestTipManager_TimeSinceConfirmation_Confirmed(t *testing.T) {
	tangle := NewTestTangle()
	tangle.Booker.MarkersManager.Manager = markers.NewManager(markers.WithCacheTime(0), markers.WithMaxPastMarkerDistance(10))

	defer tangle.Shutdown()

	tipManager := tangle.TipManager
	confirmationOracle := &MockConfirmationOracleTipManagerTest{
		confirmedBlockIDs:      NewBlockIDs(),
		confirmedMarkers:       markers.NewMarkers(),
		MockConfirmationOracle: MockConfirmationOracle{},
	}
	tangle.ConfirmationOracle = confirmationOracle

	testFramework := NewBlockTestFramework(
		tangle,
	)

	tangle.Setup()

	createTestTangleTSC(t, testFramework)
	confirmedBlockIDsString := []string{"Marker-0/1", "0/1-preTSCSeq1_0", "0/1-preTSCSeq1_1", "0/1-preTSCSeq1_2", "0/1-postTSCSeq1_0", "0/1-postTSCSeq1_1", "0/1-postTSCSeq1_2", "0/1-postTSCSeq1_3", "0/1-postTSCSeq1_4", "0/1-postTSCSeq1_5", "Marker-1/2", "0/1-preTSCSeq2_0", "0/1-preTSCSeq2_1", "0/1-preTSCSeq2_2", "0/1-postTSCSeq2_0", "0/1-postTSCSeq2_1", "0/1-postTSCSeq2_2", "0/1-postTSCSeq2_3", "0/1-postTSCSeq2_4", "0/1-postTSCSeq2_5", "Marker-2/2", "2/2_0", "2/2_1", "2/2_2", "2/2_3", "2/2_4", "Marker-2/3"}
	confirmedBlockIDs := prepareConfirmedBlockIDs(testFramework, confirmedBlockIDsString)
	confirmedMarkers := markers.NewMarkers(markers.NewMarker(0, 1), markers.NewMarker(1, 2), markers.NewMarker(2, 3))

	confirmationOracle.Lock()
	confirmationOracle.confirmedBlockIDs = confirmedBlockIDs
	confirmationOracle.confirmedMarkers = confirmedMarkers
	confirmationOracle.Unlock()

	// = &MockConfirmationOracleTipManagerTest{confirmedBlockIDs: confirmedBlockIDs, confirmedMarkers: confirmedMarkers}
	tangle.TimeManager.updateTime(testFramework.Block("Marker-2/3"))
	tangle.TimeManager.updateSyncedState()

	// Even without any confirmations, it should be possible to attach to genesis.
	assert.True(t, tipManager.isPastConeTimestampCorrect(EmptyBlockID))

	// case 0 - only one block can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/0_2").ID()))
	// case #1
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/3_4").ID()))
	// case #2
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("1/3_4").ID()))
	// case #3
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3_4").ID()))
	// case #4 (marker block)
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-1/2").ID()))
	// case #5
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/5_4").ID()))
	// case #6 (attach to unconfirmed block older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSC_2").ID()))
	// case #7
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("3/2_4").ID()))
	// case #8
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3+0/4_3").ID()))
	// case #9
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-4/5").ID()))
	// case #10 (attach to confirmed block older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq2_2").ID()))
	// case #11
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("5/2_4").ID()))
	// case #12 (attach to 0/1-postTSCSeq3_4)
	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSCSeq3_4").ID()))
	// case #13 (attach to unconfirmed block younger than TSC, with confirmed past marker older than TSC)
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC_2").ID()))
	// case #14
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("6/2_4").ID()))
	// case #15
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq5_4").ID()))
	// case #16
	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC-direct_0").ID()))
}

func createTestTangleTSC(t *testing.T, testFramework *BlockTestFramework) {
	var lastBlkAlias string

	// SEQUENCE 0
	{
		testFramework.CreateBlock("Marker-0/1", WithStrongParents("Genesis"), WithIssuingTime(time.Now().Add(-9*time.Minute)))
		testFramework.IssueBlocks("Marker-0/1").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSC", 3, []string{"Marker-0/1"}, time.Minute*8)
		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSC", 3, []string{lastBlkAlias}, time.Minute)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-0/2", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-0/2").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/2", 5, []string{"Marker-0/2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-0/3", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-0/3").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/3", 5, []string{"Marker-0/3"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-0/4", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-0/4").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		_ = issueBlocks(testFramework, "0/4", 5, []string{"Marker-0/4"}, 0)
		testFramework.PreventNewMarkers(false)

		// issue block for test case #16
		testFramework.CreateBlock("0/1-postTSC-direct_0", WithStrongParents("Marker-0/1"))
		testFramework.IssueBlocks("0/1-postTSC-direct_0").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Marker-0/1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSC_0":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSC_1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSC_2":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSC_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSC_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSC_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-0/2":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Marker-0/3":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Marker-0/4":    markers.NewMarkers(markers.NewMarker(0, 4)),
		})
	}

	// SEQUENCE 0 (without markers)
	{
		_ = issueBlocks(testFramework, "0/0", 3, []string{"Genesis"}, time.Minute)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/0_0": markers.NewMarkers(markers.NewMarker(0, 0)),
			"0/0_1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"0/0_2": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
	}
	// SEQUENCE 1
	{ //nolint:dupl
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSCSeq1", 3, []string{"Marker-0/1"}, time.Minute*6)
		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq1", 6, []string{lastBlkAlias}, time.Minute*4)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-1/2", WithStrongParents(lastBlkAlias), WithIssuingTime(time.Now().Add(-3*time.Minute)))
		testFramework.IssueBlocks("Marker-1/2").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "1/2", 5, []string{"Marker-1/2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-1/3", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-1/3").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		_ = issueBlocks(testFramework, "1/3", 5, []string{"Marker-1/3"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-preTSCSeq1_0":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq1_1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq1_2":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq1_5": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-1/2":        markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_0":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_1":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_2":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_3":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"1/2_4":             markers.NewMarkers(markers.NewMarker(1, 2)),
			"Marker-1/3":        markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_0":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_1":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_2":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_3":             markers.NewMarkers(markers.NewMarker(1, 3)),
			"1/3_4":             markers.NewMarkers(markers.NewMarker(1, 3)),
		})
	}

	// SEQUENCE 2
	{ //nolint:dupl
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSCSeq2", 3, []string{"Marker-0/1"}, time.Minute*6)
		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq2", 6, []string{lastBlkAlias}, time.Minute*4)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-2/2", WithStrongParents(lastBlkAlias), WithIssuingTime(time.Now().Add(-3*time.Minute)))
		testFramework.IssueBlocks("Marker-2/2").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "2/2", 5, []string{"Marker-2/2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-2/3", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-2/3").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		_ = issueBlocks(testFramework, "2/3", 5, []string{"Marker-2/3"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-preTSCSeq2_0":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq2_1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq2_2":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq2_5": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-2/2":        markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_0":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_1":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_2":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_3":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"2/2_4":             markers.NewMarkers(markers.NewMarker(2, 2)),
			"Marker-2/3":        markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_0":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_1":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_2":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_3":             markers.NewMarkers(markers.NewMarker(2, 3)),
			"2/3_4":             markers.NewMarkers(markers.NewMarker(2, 3)),
		})
	}

	// SEQUENCE 2 + 0
	{
		testFramework.CreateBlock("Marker-2/5", WithStrongParents("0/4_4", "2/3_4"))
		testFramework.IssueBlocks("Marker-2/5").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		_ = issueBlocks(testFramework, "2/5", 5, []string{"Marker-2/5"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Marker-2/5": markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_0":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_1":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_2":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_3":      markers.NewMarkers(markers.NewMarker(2, 5)),
			"2/5_4":      markers.NewMarkers(markers.NewMarker(2, 5)),
		})
	}

	// SEQUENCE 3
	{
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq3", 5, []string{"0/1-postTSCSeq2_0"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-3/2", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-3/2").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		_ = issueBlocks(testFramework, "3/2", 5, []string{"Marker-3/2"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-postTSCSeq3_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq3_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-3/2":        markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_0":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_1":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_2":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_3":             markers.NewMarkers(markers.NewMarker(3, 2)),
			"3/2_4":             markers.NewMarkers(markers.NewMarker(3, 2)),
		})
	}

	// SEQUENCE 2 + 0 (two past markers) -> SEQUENCE 4
	{
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "2/3+0/4", 5, []string{"0/4_4", "2/3_4"}, 0)
		testFramework.CreateBlock("Marker-4/5", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-4/5").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"2/3+0/4_0":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"2/3+0/4_1":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"2/3+0/4_2":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"2/3+0/4_3":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(0, 4)),
			"Marker-4/5": markers.NewMarkers(markers.NewMarker(4, 5)),
		})
	}
	// SEQUENCE 5
	{
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSCSeq5", 6, []string{"0/1-preTSCSeq2_2"}, time.Minute*6)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-5/2", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-5/2").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		_ = issueBlocks(testFramework, "5/2", 5, []string{"Marker-5/2"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-preTSCSeq5_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-preTSCSeq5_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-5/2":       markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_0":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_1":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_2":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_3":            markers.NewMarkers(markers.NewMarker(5, 2)),
			"5/2_4":            markers.NewMarkers(markers.NewMarker(5, 2)),
		})
	}

	// SEQUENCE 6
	{
		testFramework.PreventNewMarkers(true)
		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq6", 6, []string{"0/1-preTSCSeq2_2"}, 0)
		testFramework.PreventNewMarkers(false)
		testFramework.CreateBlock("Marker-6/2", WithStrongParents(lastBlkAlias))
		testFramework.IssueBlocks("Marker-6/2").WaitUntilAllTasksProcessed()
		testFramework.PreventNewMarkers(true)
		_ = issueBlocks(testFramework, "6/2", 5, []string{"Marker-6/2"}, 0)
		testFramework.PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"0/1-postTSCSeq6_0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq6_1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq6_2": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq6_3": markers.NewMarkers(markers.NewMarker(0, 1)),
			"0/1-postTSCSeq6_4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Marker-6/2":        markers.NewMarkers(markers.NewMarker(6, 2)),
			"6/2_0":             markers.NewMarkers(markers.NewMarker(6, 2)),
			"6/2_1":             markers.NewMarkers(markers.NewMarker(6, 2)),
			"6/2_2":             markers.NewMarkers(markers.NewMarker(6, 2)),
			"6/2_3":             markers.NewMarkers(markers.NewMarker(6, 2)),
			"6/2_4":             markers.NewMarkers(markers.NewMarker(6, 2)),
		})
	}
}

func prepareConfirmedBlockIDs(testFramework *BlockTestFramework, confirmedIDs []string) BlockIDs {
	confirmedBlockIDs := NewBlockIDs()
	for _, id := range confirmedIDs {
		confirmedBlockIDs.Add(testFramework.Block(id).ID())
	}
	return confirmedBlockIDs
}

func issueBlocks(testFramework *BlockTestFramework, blkPrefix string, blkCount int, parents []string, timestampOffset time.Duration) string {
	blkAlias := fmt.Sprintf("%s_%d", blkPrefix, 0)

	testFramework.CreateBlock(blkAlias, WithStrongParents(parents...), WithIssuingTime(time.Now().Add(-timestampOffset)))
	testFramework.IssueBlocks(blkAlias).WaitUntilAllTasksProcessed()

	for i := 1; i < blkCount; i++ {
		alias := fmt.Sprintf("%s_%d", blkPrefix, i)
		testFramework.CreateBlock(alias, WithIssuer(identity.GenerateIdentity().PublicKey()), WithStrongParents(blkAlias), WithSequenceNumber(uint64(i)), WithIssuingTime(time.Now().Add(-timestampOffset)))
		testFramework.IssueBlocks(alias).WaitUntilAllTasksProcessed()
		fmt.Println("issuing block", testFramework.Block(alias).ID())
		blkAlias = alias
	}
	return blkAlias
}

func createAndStoreParentsDataBlockInMasterConflict(tangle *Tangle, strongParents, weakParents BlockIDs) (block *Block) {
	parents := ParentBlockIDs{
		StrongParentType: strongParents,
	}
	if len(weakParents) > 0 {
		parents[WeakParentType] = weakParents
	}

	block = newTestParentsDataBlock("testblock", parents)
	tangle.Storage.StoreBlock(block)
	event.Loop.WaitUntilAllTasksProcessed()

	return
}

type MockConfirmationOracleTipManagerTest struct {
	confirmedBlockIDs BlockIDs
	confirmedMarkers  *markers.Markers

	MockConfirmationOracle
}

// IsBlockConfirmed mocks its interface function.
func (m *MockConfirmationOracleTipManagerTest) IsBlockConfirmed(blkID BlockID) bool {
	m.RLock()
	defer m.RUnlock()

	return m.confirmedBlockIDs.Contains(blkID)
}

// FirstUnconfirmedMarkerIndex mocks its interface function.
func (m *MockConfirmationOracleTipManagerTest) FirstUnconfirmedMarkerIndex(sequenceID markers.SequenceID) (unconfirmedMarkerIndex markers.Index) {
	m.RLock()
	defer m.RUnlock()

	confirmedMarkerIndex, exists := m.confirmedMarkers.Get(sequenceID)
	if exists {
		return confirmedMarkerIndex + 1
	}
	return 0
}

// IsBlockConfirmed mocks its interface function.
func (m *MockConfirmationOracleTipManagerTest) IsMarkerConfirmed(marker markers.Marker) bool {
	m.RLock()
	defer m.RUnlock()

	if m.confirmedMarkers == nil || m.confirmedMarkers.Size() == 0 {
		return false
	}
	confirmedMarkerIndex, exists := m.confirmedMarkers.Get(marker.SequenceID())
	if !exists {
		return false
	}
	return marker.Index() <= confirmedMarkerIndex
}
