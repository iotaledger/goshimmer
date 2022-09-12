package tip

import (
	"testing"
)

func TestTipManager_DataBlockTips(t *testing.T) {
	tf := NewTestFramework(t)
	tipManager := tf.TipManager

	// without any tip -> genesis
	{
		tf.AssertTips(tipManager.Tips(2), "Genesis")
	}

	// Block 1
	{
		tf.CreateBlock("1")
		tf.IssueBlocks("1").WaitUntilAllTasksProcessed()

		tf.AssertTipCount(1)
		tf.AssertTips(tipManager.Tips(2), "1")
	}
	// fmt.Println("send blk2")
	// // Block 2
	// {
	// 	blocks["2"] = createAndStoreParentsDataBlockInMasterConflict(tangle, NewBlockIDs(EmptyBlockID), NewBlockIDs())
	// 	tipManager.AddTip(blocks["2"])
	//
	// 	assert.Equal(t, 2, tipManager.TipCount())
	// 	assert.Contains(t, tipManager.tips.Keys(), blocks["1"].ID(), blocks["2"].ID())
	//
	// 	parents := tipManager.Tips(nil, 3)
	// 	assert.Len(t, parents, 2)
	// 	assert.Contains(t, parents, blocks["1"].ID(), blocks["2"].ID())
	// }
	// fmt.Println("send blk3")
	// // Block 3
	// {
	// 	blocks["3"] = createAndStoreParentsDataBlockInMasterConflict(tangle, NewBlockIDs(blocks["1"].ID(), blocks["2"].ID()), NewBlockIDs())
	// 	tipManager.AddTip(blocks["3"])
	//
	// 	assert.Equal(t, 1, tipManager.TipCount())
	// 	assert.Contains(t, tipManager.tips.Keys(), blocks["3"].ID())
	//
	// 	parents := tipManager.Tips(nil, 2)
	// 	assert.Len(t, parents, 1)
	// 	assert.Contains(t, parents, blocks["3"].ID())
	// }
	// fmt.Println("send blk3")
	// // Add Block 4-8
	// {
	// 	tips := NewBlockIDs()
	// 	tips.Add(blocks["3"].ID())
	// 	for count, n := range []int{4, 5, 6, 7, 8} {
	// 		nString := strconv.Itoa(n)
	// 		blocks[nString] = createAndStoreParentsDataBlockInMasterConflict(tangle, NewBlockIDs(blocks["1"].ID()), NewBlockIDs())
	// 		tipManager.AddTip(blocks[nString])
	// 		tips.Add(blocks[nString].ID())
	//
	// 		assert.Equalf(t, count+2, tipManager.TipCount(), "TipCount does not match after adding Block %d", n)
	// 		assert.ElementsMatchf(t, tipManager.tips.Keys(), tips.Slice(), "Elements in strongTips do not match after adding Block %d", n)
	// 		assert.Contains(t, tipManager.tips.Keys(), blocks["3"].ID())
	// 		fmt.Println("send blk", n)
	// 	}
	// }
	//
	// // now we have 6 tips
	// // Tips(4) -> 4
	// {
	// 	parents := tipManager.Tips(nil, 4)
	// 	assert.Len(t, parents, 4)
	// }
	// fmt.Println("select tip1")
	// // Tips(8) -> 6
	// {
	// 	parents := tipManager.Tips(nil, 8)
	// 	assert.Len(t, parents, 6)
	// }
	// fmt.Println("select tip2")
	// // Tips(0) -> 1
	// {
	// 	parents := tipManager.Tips(nil, 0)
	// 	assert.Len(t, parents, 1)
	// }
	// fmt.Println("select tip3")
}

// Test based on packages/tangle/images/TSC_test_scenario.png except nothing is confirmed.
// func TestTipManager_TimeSinceConfirmation_Unconfirmed(t *testing.T) {
// 	t.Skip("Skip this test.")
// 	tangle := NewTestTangle()
// 	tangle.Booker.MarkersManager.Manager = markersold.NewManager(markersold.WithCacheTime(0), markersold.WithMaxPastMarkerDistance(10))
// 	defer tangle.Shutdown()
//
// 	tipManager := tangle.TipManager
//
// 	testFramework := NewBlockTestFramework(
// 		tangle,
// 	)
//
// 	tangle.Setup()
//
// 	createTestTangleTSC(t, testFramework)
// 	var confirmedBlockIDsString []string
// 	confirmedBlockIDs := prepareConfirmedBlockIDs(testFramework, confirmedBlockIDsString)
// 	confirmedMarkers := markersold.NewMarkers()
//
// 	tangle.ConfirmationOracle = &MockConfirmationOracleTipManagerTest{confirmedBlockIDs: confirmedBlockIDs, confirmedMarkers: confirmedMarkers}
// 	tangle.TimeManager.updateTime(testFramework.Block("Marker-2/3"))
// 	tangle.TimeManager.updateSyncedState()
//
// 	// Even without any confirmations, it should be possible to attach to genesis.
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(EmptyBlockID))
//
// 	// case 0 - only one block can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/0_2").ID()))
// 	// case #1
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/3_4").ID()))
// 	// case #2
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("1/3_4").ID()))
// 	// case #3
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3_4").ID()))
// 	// case #4 (marker block)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-1/2").ID()))
// 	// case #5
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/5_4").ID()))
// 	// case #6 (attach to unconfirmed block older than TSC)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSC_2").ID()))
// 	// case #7
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("3/2_4").ID()))
// 	// case #8
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3+0/4_3").ID()))
// 	// case #9
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-4/5").ID()))
// 	// case #10 (attach to confirmed block older than TSC)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq2_2").ID()))
// 	// case #11
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("5/2_4").ID()))
// 	// case #12 (attach to 0/1-postTSCSeq3_4)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSCSeq3_4").ID()))
// 	// case #13 (attach to unconfirmed block younger than TSC, with confirmed past marker older than TSC)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC_2").ID()))
// 	// case #14
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("6/2_4").ID()))
// 	// case #15
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq5_4").ID()))
// 	// case #16
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC-direct_0").ID()))
// }
//
// // Test based on packages/tangle/images/TSC_test_scenario.png.
// func TestTipManager_TimeSinceConfirmation_Confirmed(t *testing.T) {
// 	tangle := NewTestTangle()
// 	tangle.Booker.MarkersManager.Manager = markersold.NewManager(markersold.WithCacheTime(0), markersold.WithMaxPastMarkerDistance(10))
//
// 	defer tangle.Shutdown()
//
// 	tipManager := tangle.TipManager
// 	confirmationOracle := &MockConfirmationOracleTipManagerTest{
// 		confirmedBlockIDs:      NewBlockIDs(),
// 		confirmedMarkers:       markersold.NewMarkers(),
// 		MockConfirmationOracle: MockConfirmationOracle{},
// 	}
// 	tangle.ConfirmationOracle = confirmationOracle
//
// 	testFramework := NewBlockTestFramework(
// 		tangle,
// 	)
//
// 	tangle.Setup()
//
// 	createTestTangleTSC(t, testFramework)
// 	confirmedBlockIDsString := []string{"Marker-0/1", "0/1-preTSCSeq1_0", "0/1-preTSCSeq1_1", "0/1-preTSCSeq1_2", "0/1-postTSCSeq1_0", "0/1-postTSCSeq1_1", "0/1-postTSCSeq1_2", "0/1-postTSCSeq1_3", "0/1-postTSCSeq1_4", "0/1-postTSCSeq1_5", "Marker-1/2", "0/1-preTSCSeq2_0", "0/1-preTSCSeq2_1", "0/1-preTSCSeq2_2", "0/1-postTSCSeq2_0", "0/1-postTSCSeq2_1", "0/1-postTSCSeq2_2", "0/1-postTSCSeq2_3", "0/1-postTSCSeq2_4", "0/1-postTSCSeq2_5", "Marker-2/2", "2/2_0", "2/2_1", "2/2_2", "2/2_3", "2/2_4", "Marker-2/3"}
// 	confirmedBlockIDs := prepareConfirmedBlockIDs(testFramework, confirmedBlockIDsString)
// 	confirmedMarkers := markersold.NewMarkers(markersold.NewMarker(0, 1), markersold.NewMarker(1, 2), markersold.NewMarker(2, 3))
//
// 	confirmationOracle.Lock()
// 	confirmationOracle.confirmedBlockIDs = confirmedBlockIDs
// 	confirmationOracle.confirmedMarkers = confirmedMarkers
// 	confirmationOracle.Unlock()
//
// 	// = &MockConfirmationOracleTipManagerTest{confirmedBlockIDs: confirmedBlockIDs, confirmedMarkers: confirmedMarkers}
// 	tangle.TimeManager.updateTime(testFramework.Block("Marker-2/3"))
// 	tangle.TimeManager.updateSyncedState()
//
// 	// Even without any confirmations, it should be possible to attach to genesis.
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(EmptyBlockID))
//
// 	// case 0 - only one block can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/0_2").ID()))
// 	// case #1
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/3_4").ID()))
// 	// case #2
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("1/3_4").ID()))
// 	// case #3
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3_4").ID()))
// 	// case #4 (marker block)
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-1/2").ID()))
// 	// case #5
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/5_4").ID()))
// 	// case #6 (attach to unconfirmed block older than TSC)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSC_2").ID()))
// 	// case #7
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("3/2_4").ID()))
// 	// case #8
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("2/3+0/4_3").ID()))
// 	// case #9
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("Marker-4/5").ID()))
// 	// case #10 (attach to confirmed block older than TSC)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq2_2").ID()))
// 	// case #11
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("5/2_4").ID()))
// 	// case #12 (attach to 0/1-postTSCSeq3_4)
// 	assert.True(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSCSeq3_4").ID()))
// 	// case #13 (attach to unconfirmed block younger than TSC, with confirmed past marker older than TSC)
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC_2").ID()))
// 	// case #14
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("6/2_4").ID()))
// 	// case #15
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-preTSCSeq5_4").ID()))
// 	// case #16
// 	assert.False(t, tipManager.isPastConeTimestampCorrect(testFramework.Block("0/1-postTSC-direct_0").ID()))
// }
//
// func createTestTangleTSC(t *testing.T, testFramework *BlockTestFramework) {
// 	var lastBlkAlias string
//
// 	// SEQUENCE 0
// 	{
// 		testFramework.CreateBlock("Marker-0/1", WithStrongParents("Genesis"), WithIssuingTime(time.Now().Add(-9*time.Minute)))
// 		testFramework.IssueBlocks("Marker-0/1").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSC", 3, []string{"Marker-0/1"}, time.Minute*8)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSC", 3, []string{lastBlkAlias}, time.Minute)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-0/2", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-0/2").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/2", 5, []string{"Marker-0/2"}, 0)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-0/3", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-0/3").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/3", 5, []string{"Marker-0/3"}, 0)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-0/4", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-0/4").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		_ = issueBlocks(testFramework, "0/4", 5, []string{"Marker-0/4"}, 0)
// 		testFramework.PreventNewMarkers(false)
//
// 		// issue block for test case #16
// 		testFramework.CreateBlock("0/1-postTSC-direct_0", WithStrongParents("Marker-0/1"))
// 		testFramework.IssueBlocks("0/1-postTSC-direct_0").WaitUntilAllTasksProcessed()
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"Marker-0/1":    markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSC_0":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSC_1":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSC_2":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSC_0": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSC_1": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSC_2": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"Marker-0/2":    markersold.NewMarkers(markersold.NewMarker(0, 2)),
// 			"Marker-0/3":    markersold.NewMarkers(markersold.NewMarker(0, 3)),
// 			"Marker-0/4":    markersold.NewMarkers(markersold.NewMarker(0, 4)),
// 		})
// 	}
//
// 	// SEQUENCE 0 (without markers)
// 	{
// 		_ = issueBlocks(testFramework, "0/0", 3, []string{"Genesis"}, time.Minute)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"0/0_0": markersold.NewMarkers(markersold.NewMarker(0, 0)),
// 			"0/0_1": markersold.NewMarkers(markersold.NewMarker(0, 0)),
// 			"0/0_2": markersold.NewMarkers(markersold.NewMarker(0, 0)),
// 		})
// 	}
// 	// SEQUENCE 1
// 	{ //nolint:dupl
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSCSeq1", 3, []string{"Marker-0/1"}, time.Minute*6)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq1", 6, []string{lastBlkAlias}, time.Minute*4)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-1/2", WithStrongParents(lastBlkAlias), WithIssuingTime(time.Now().Add(-3*time.Minute)))
// 		testFramework.IssueBlocks("Marker-1/2").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "1/2", 5, []string{"Marker-1/2"}, 0)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-1/3", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-1/3").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		_ = issueBlocks(testFramework, "1/3", 5, []string{"Marker-1/3"}, 0)
// 		testFramework.PreventNewMarkers(false)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"0/1-preTSCSeq1_0":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq1_1":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq1_2":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq1_0": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq1_1": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq1_2": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq1_3": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq1_4": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq1_5": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"Marker-1/2":        markersold.NewMarkers(markersold.NewMarker(1, 2)),
// 			"1/2_0":             markersold.NewMarkers(markersold.NewMarker(1, 2)),
// 			"1/2_1":             markersold.NewMarkers(markersold.NewMarker(1, 2)),
// 			"1/2_2":             markersold.NewMarkers(markersold.NewMarker(1, 2)),
// 			"1/2_3":             markersold.NewMarkers(markersold.NewMarker(1, 2)),
// 			"1/2_4":             markersold.NewMarkers(markersold.NewMarker(1, 2)),
// 			"Marker-1/3":        markersold.NewMarkers(markersold.NewMarker(1, 3)),
// 			"1/3_0":             markersold.NewMarkers(markersold.NewMarker(1, 3)),
// 			"1/3_1":             markersold.NewMarkers(markersold.NewMarker(1, 3)),
// 			"1/3_2":             markersold.NewMarkers(markersold.NewMarker(1, 3)),
// 			"1/3_3":             markersold.NewMarkers(markersold.NewMarker(1, 3)),
// 			"1/3_4":             markersold.NewMarkers(markersold.NewMarker(1, 3)),
// 		})
// 	}
//
// 	// SEQUENCE 2
// 	{ //nolint:dupl
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSCSeq2", 3, []string{"Marker-0/1"}, time.Minute*6)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq2", 6, []string{lastBlkAlias}, time.Minute*4)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-2/2", WithStrongParents(lastBlkAlias), WithIssuingTime(time.Now().Add(-3*time.Minute)))
// 		testFramework.IssueBlocks("Marker-2/2").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "2/2", 5, []string{"Marker-2/2"}, 0)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-2/3", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-2/3").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		_ = issueBlocks(testFramework, "2/3", 5, []string{"Marker-2/3"}, 0)
// 		testFramework.PreventNewMarkers(false)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"0/1-preTSCSeq2_0":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq2_1":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq2_2":  markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq2_0": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq2_1": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq2_2": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq2_3": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq2_4": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq2_5": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"Marker-2/2":        markersold.NewMarkers(markersold.NewMarker(2, 2)),
// 			"2/2_0":             markersold.NewMarkers(markersold.NewMarker(2, 2)),
// 			"2/2_1":             markersold.NewMarkers(markersold.NewMarker(2, 2)),
// 			"2/2_2":             markersold.NewMarkers(markersold.NewMarker(2, 2)),
// 			"2/2_3":             markersold.NewMarkers(markersold.NewMarker(2, 2)),
// 			"2/2_4":             markersold.NewMarkers(markersold.NewMarker(2, 2)),
// 			"Marker-2/3":        markersold.NewMarkers(markersold.NewMarker(2, 3)),
// 			"2/3_0":             markersold.NewMarkers(markersold.NewMarker(2, 3)),
// 			"2/3_1":             markersold.NewMarkers(markersold.NewMarker(2, 3)),
// 			"2/3_2":             markersold.NewMarkers(markersold.NewMarker(2, 3)),
// 			"2/3_3":             markersold.NewMarkers(markersold.NewMarker(2, 3)),
// 			"2/3_4":             markersold.NewMarkers(markersold.NewMarker(2, 3)),
// 		})
// 	}
//
// 	// SEQUENCE 2 + 0
// 	{
// 		testFramework.CreateBlock("Marker-2/5", WithStrongParents("0/4_4", "2/3_4"))
// 		testFramework.IssueBlocks("Marker-2/5").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		_ = issueBlocks(testFramework, "2/5", 5, []string{"Marker-2/5"}, 0)
// 		testFramework.PreventNewMarkers(false)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"Marker-2/5": markersold.NewMarkers(markersold.NewMarker(2, 5)),
// 			"2/5_0":      markersold.NewMarkers(markersold.NewMarker(2, 5)),
// 			"2/5_1":      markersold.NewMarkers(markersold.NewMarker(2, 5)),
// 			"2/5_2":      markersold.NewMarkers(markersold.NewMarker(2, 5)),
// 			"2/5_3":      markersold.NewMarkers(markersold.NewMarker(2, 5)),
// 			"2/5_4":      markersold.NewMarkers(markersold.NewMarker(2, 5)),
// 		})
// 	}
//
// 	// SEQUENCE 3
// 	{
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq3", 5, []string{"0/1-postTSCSeq2_0"}, 0)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-3/2", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-3/2").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		_ = issueBlocks(testFramework, "3/2", 5, []string{"Marker-3/2"}, 0)
// 		testFramework.PreventNewMarkers(false)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"0/1-postTSCSeq3_0": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq3_1": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq3_2": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq3_3": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq3_4": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"Marker-3/2":        markersold.NewMarkers(markersold.NewMarker(3, 2)),
// 			"3/2_0":             markersold.NewMarkers(markersold.NewMarker(3, 2)),
// 			"3/2_1":             markersold.NewMarkers(markersold.NewMarker(3, 2)),
// 			"3/2_2":             markersold.NewMarkers(markersold.NewMarker(3, 2)),
// 			"3/2_3":             markersold.NewMarkers(markersold.NewMarker(3, 2)),
// 			"3/2_4":             markersold.NewMarkers(markersold.NewMarker(3, 2)),
// 		})
// 	}
//
// 	// SEQUENCE 2 + 0 (two past markers) -> SEQUENCE 4
// 	{
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "2/3+0/4", 5, []string{"0/4_4", "2/3_4"}, 0)
// 		testFramework.CreateBlock("Marker-4/5", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-4/5").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(false)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"2/3+0/4_0":  markersold.NewMarkers(markersold.NewMarker(2, 3), markersold.NewMarker(0, 4)),
// 			"2/3+0/4_1":  markersold.NewMarkers(markersold.NewMarker(2, 3), markersold.NewMarker(0, 4)),
// 			"2/3+0/4_2":  markersold.NewMarkers(markersold.NewMarker(2, 3), markersold.NewMarker(0, 4)),
// 			"2/3+0/4_3":  markersold.NewMarkers(markersold.NewMarker(2, 3), markersold.NewMarker(0, 4)),
// 			"Marker-4/5": markersold.NewMarkers(markersold.NewMarker(4, 5)),
// 		})
// 	}
// 	// SEQUENCE 5
// 	{
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-preTSCSeq5", 6, []string{"0/1-preTSCSeq2_2"}, time.Minute*6)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-5/2", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-5/2").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		_ = issueBlocks(testFramework, "5/2", 5, []string{"Marker-5/2"}, 0)
// 		testFramework.PreventNewMarkers(false)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"0/1-preTSCSeq5_0": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq5_1": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq5_2": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq5_3": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-preTSCSeq5_4": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"Marker-5/2":       markersold.NewMarkers(markersold.NewMarker(5, 2)),
// 			"5/2_0":            markersold.NewMarkers(markersold.NewMarker(5, 2)),
// 			"5/2_1":            markersold.NewMarkers(markersold.NewMarker(5, 2)),
// 			"5/2_2":            markersold.NewMarkers(markersold.NewMarker(5, 2)),
// 			"5/2_3":            markersold.NewMarkers(markersold.NewMarker(5, 2)),
// 			"5/2_4":            markersold.NewMarkers(markersold.NewMarker(5, 2)),
// 		})
// 	}
//
// 	// SEQUENCE 6
// 	{
// 		testFramework.PreventNewMarkers(true)
// 		lastBlkAlias = issueBlocks(testFramework, "0/1-postTSCSeq6", 6, []string{"0/1-preTSCSeq2_2"}, 0)
// 		testFramework.PreventNewMarkers(false)
// 		testFramework.CreateBlock("Marker-6/2", WithStrongParents(lastBlkAlias))
// 		testFramework.IssueBlocks("Marker-6/2").WaitUntilAllTasksProcessed()
// 		testFramework.PreventNewMarkers(true)
// 		_ = issueBlocks(testFramework, "6/2", 5, []string{"Marker-6/2"}, 0)
// 		testFramework.PreventNewMarkers(false)
//
// 		checkMarkers(t, testFramework, map[string]*markersold.Markers{
// 			"0/1-postTSCSeq6_0": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq6_1": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq6_2": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq6_3": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"0/1-postTSCSeq6_4": markersold.NewMarkers(markersold.NewMarker(0, 1)),
// 			"Marker-6/2":        markersold.NewMarkers(markersold.NewMarker(6, 2)),
// 			"6/2_0":             markersold.NewMarkers(markersold.NewMarker(6, 2)),
// 			"6/2_1":             markersold.NewMarkers(markersold.NewMarker(6, 2)),
// 			"6/2_2":             markersold.NewMarkers(markersold.NewMarker(6, 2)),
// 			"6/2_3":             markersold.NewMarkers(markersold.NewMarker(6, 2)),
// 			"6/2_4":             markersold.NewMarkers(markersold.NewMarker(6, 2)),
// 		})
// 	}
// }
//
// func prepareConfirmedBlockIDs(testFramework *BlockTestFramework, confirmedIDs []string) BlockIDs {
// 	confirmedBlockIDs := NewBlockIDs()
// 	for _, id := range confirmedIDs {
// 		confirmedBlockIDs.Add(testFramework.Block(id).ID())
// 	}
// 	return confirmedBlockIDs
// }
//
// func issueBlocks(testFramework *BlockTestFramework, blkPrefix string, blkCount int, parents []string, timestampOffset time.Duration) string {
// 	blkAlias := fmt.Sprintf("%s_%d", blkPrefix, 0)
//
// 	testFramework.CreateBlock(blkAlias, WithStrongParents(parents...), WithIssuingTime(time.Now().Add(-timestampOffset)))
// 	testFramework.IssueBlocks(blkAlias).WaitUntilAllTasksProcessed()
//
// 	for i := 1; i < blkCount; i++ {
// 		alias := fmt.Sprintf("%s_%d", blkPrefix, i)
// 		testFramework.CreateBlock(alias, WithIssuer(identity.GenerateIdentity().PublicKey()), WithStrongParents(blkAlias), WithSequenceNumber(uint64(i)), WithIssuingTime(time.Now().Add(-timestampOffset)))
// 		testFramework.IssueBlocks(alias).WaitUntilAllTasksProcessed()
// 		fmt.Println("issuing block", testFramework.Block(alias).ID())
// 		blkAlias = alias
// 	}
// 	return blkAlias
// }
//
// func createAndStoreParentsDataBlockInMasterConflict(tangle *Tangle, strongParents, weakParents BlockIDs) (block *Block) {
// 	parents := ParentBlockIDs{
// 		StrongParentType: strongParents,
// 	}
// 	if len(weakParents) > 0 {
// 		parents[WeakParentType] = weakParents
// 	}
//
// 	block = newTestParentsDataBlock("testblock", parents)
// 	tangle.Storage.StoreBlock(block)
// 	event.Loop.WaitUntilAllTasksProcessed()
//
// 	return
// }
//
// type MockConfirmationOracleTipManagerTest struct {
// 	confirmedBlockIDs BlockIDs
// 	confirmedMarkers  *markersold.Markers
//
// 	MockConfirmationOracle
// }
//
// // IsBlockConfirmed mocks its interface function.
// func (m *MockConfirmationOracleTipManagerTest) IsBlockConfirmed(blkID BlockID) bool {
// 	m.RLock()
// 	defer m.RUnlock()
//
// 	return m.confirmedBlockIDs.Contains(blkID)
// }
//
// // FirstUnconfirmedMarkerIndex mocks its interface function.
// func (m *MockConfirmationOracleTipManagerTest) FirstUnconfirmedMarkerIndex(sequenceID markersold.SequenceID) (unconfirmedMarkerIndex markersold.Index) {
// 	m.RLock()
// 	defer m.RUnlock()
//
// 	confirmedMarkerIndex, exists := m.confirmedMarkers.Get(sequenceID)
// 	if exists {
// 		return confirmedMarkerIndex + 1
// 	}
// 	return 0
// }
//
// // IsBlockConfirmed mocks its interface function.
// func (m *MockConfirmationOracleTipManagerTest) IsMarkerConfirmed(marker markersold.Marker) bool {
// 	m.RLock()
// 	defer m.RUnlock()
//
// 	if m.confirmedMarkers == nil || m.confirmedMarkers.Size() == 0 {
// 		return false
// 	}
// 	confirmedMarkerIndex, exists := m.confirmedMarkers.Get(marker.SequenceID())
// 	if !exists {
// 		return false
// 	}
// 	return marker.Index() <= confirmedMarkerIndex
// }
