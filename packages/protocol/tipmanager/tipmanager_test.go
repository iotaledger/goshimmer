//nolint:dupl
package tipmanager

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization/slotnotarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func TestTipManager_DataBlockTips(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t,
		workers.CreateGroup("TipManagerTestFramework"),
	)

	tipManager := tf.Instance

	// without any tip -> genesis
	{
		tf.AssertEqualBlocks(tipManager.Tips(2), tf.Tangle.BlockDAG.BlockIDs("Genesis"))
	}

	// Block 1
	{
		tf.Tangle.BlockDAG.CreateBlock("Block1")
		tf.Tangle.BlockDAG.IssueBlocks("Block1")
		workers.WaitChildren()

		tf.AssertTipCount(1)
		tf.AssertEqualBlocks(tipManager.Tips(2), tf.Tangle.BlockDAG.BlockIDs("Block1"))
		tf.AssertTipsAdded(1)
		tf.AssertTipsRemoved(0)
	}

	// Block 2
	{
		tf.Tangle.BlockDAG.CreateBlock("Block2")
		tf.IssueBlocksAndSetAccepted("Block2")
		workers.WaitChildren()

		tf.AssertTipCount(2)
		tf.AssertEqualBlocks(tipManager.Tips(2), tf.Tangle.BlockDAG.BlockIDs("Block1", "Block2"))
		tf.AssertTipsAdded(2)
		tf.AssertTipsRemoved(0)
	}

	// Block 3
	{
		tf.Tangle.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1", "Block2")))
		tf.IssueBlocksAndSetAccepted("Block3")
		workers.WaitChildren()

		tf.AssertTipCount(1)
		tf.AssertEqualBlocks(tipManager.Tips(2), tf.Tangle.BlockDAG.BlockIDs("Block3"))
		tf.AssertTipsAdded(3)
		tf.AssertTipsRemoved(2)
	}

	// Add Block 4-8
	{
		for count, n := range []int{4, 5, 6, 7, 8} {
			count++

			alias := fmt.Sprintf("Block%d", n)
			tf.Tangle.BlockDAG.CreateBlock(alias, models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1")))
			tf.IssueBlocksAndSetAccepted(alias)
			workers.WaitChildren()

			tf.AssertTipCount(1 + count)
			tf.AssertTipsAdded(uint32(3 + count))
			tf.AssertTipsRemoved(2)
		}
	}

	// now we have 6 tips
	// Tips(4) -> 4
	{
		parents := tipManager.Tips(4)
		require.Equal(t, 4, len(parents))
	}

	// Tips(8) -> 6
	{
		tf.AssertEqualBlocks(tipManager.Tips(8), tf.Tangle.BlockDAG.BlockIDs("Block3", "Block4", "Block5", "Block6", "Block7", "Block8"))
	}

	// Tips(0) -> 1
	{
		parents := tipManager.Tips(0)
		require.Equal(t, 1, len(parents))
	}
}

// Test based on packages/tangle/images/TSC_test_scenario.png except nothing is confirmed.
func TestTipManager_TimeSinceConfirmation_Unconfirmed(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	genesisTime := time.Now().Add(-5 * time.Hour)
	tf := NewTestFramework(t,
		workers.CreateGroup("TipManagerTestFramework"),
		WithGenesisUnixTime(genesisTime.Unix()),
		WithTipManagerOptions(
			WithTimeSinceConfirmationThreshold(5*time.Minute),
		),
		WithBookerOptions(
			markerbooker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](
					markers.WithMaxPastMarkerDistance(10),
				),
			),
		),
		WithEngineOptions(
			engine.WithBootstrapThreshold(time.Since(genesisTime.Add(-time.Hour))),
		),
	)
	tf.Engine.EvictionState.AddRootBlock(models.EmptyBlockID, commitment.ID{})

	createTestTangleTSC(tf)

	// Even without any confirmations, it should be possible to attach to genesis.
	tf.AssertIsPastConeTimestampCorrect("Genesis", true)

	// case 0 - only one block can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
	tf.AssertIsPastConeTimestampCorrect("7/1_2", true)

	tf.SetAcceptedTime(tf.Tangle.BlockDAG.Block("Marker-2/3").IssuingTime())

	// case #1
	tf.AssertIsPastConeTimestampCorrect("0/3_4", false)
	// case #2
	tf.AssertIsPastConeTimestampCorrect("1/3_4", false)
	// case #3
	tf.AssertIsPastConeTimestampCorrect("2/3_4", false)
	// case #4 (marker block)
	tf.AssertIsPastConeTimestampCorrect("Marker-1/2", false)
	// case #5
	tf.AssertIsPastConeTimestampCorrect("2/5_4", false)
	// case #6 (attach to unconfirmed block older than TSC)
	tf.AssertIsPastConeTimestampCorrect("0/1-preTSC_2", false)
	// case #7
	tf.AssertIsPastConeTimestampCorrect("3/2_4", false)
	// case #8
	tf.AssertIsPastConeTimestampCorrect("2/3+0/4_3", false)
	// case #9
	tf.AssertIsPastConeTimestampCorrect("Marker-4/5", false)
	// case #10 (attach to confirmed block older than TSC)
	tf.AssertIsPastConeTimestampCorrect("0/1-preTSCSeq2_2", false)
	// case #11
	tf.AssertIsPastConeTimestampCorrect("5/2_4", false)
	// case #12 (attach to 0/1-postTSCSeq3_4)
	tf.AssertIsPastConeTimestampCorrect("0/1-postTSCSeq3_4", false)
	// case #13 (attach to unconfirmed block younger than TSC, with confirmed past marker older than TSC)
	tf.AssertIsPastConeTimestampCorrect("0/1-postTSC_2", false)
	// case #14
	tf.AssertIsPastConeTimestampCorrect("6/2_4", false)
	// case #15
	tf.AssertIsPastConeTimestampCorrect("0/1-preTSCSeq5_4", false)
	// case #16
	tf.AssertIsPastConeTimestampCorrect("0/1-postTSC-direct_0", false)
}

// Test based on packages/tangle/images/TSC_test_scenario.png.
func TestTipManager_TimeSinceConfirmation_Confirmed(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	genesisTime := time.Now().Add(-5 * time.Hour)
	tf := NewTestFramework(t,
		workers.CreateGroup("TipManagerTestFramework"),
		WithGenesisUnixTime(genesisTime.Unix()),
		WithTipManagerOptions(
			WithTimeSinceConfirmationThreshold(5*time.Minute),
		),
		WithSlotNotarizationOptions(
			slotnotarization.WithMinCommittableSlotAge(20*6),
		),
		WithBookerOptions(
			markerbooker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](
					markers.WithMaxPastMarkerDistance(10),
				),
			),
		),
		WithEngineOptions(
			engine.WithBootstrapThreshold(time.Since(genesisTime.Add(30*time.Second))),
		),
	)
	tf.Engine.EvictionState.AddRootBlock(models.EmptyBlockID, commitment.ID{})
	createTestTangleTSC(tf)

	// case 0 - only one block can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
	tf.AssertIsPastConeTimestampCorrect("7/1_2", true)

	acceptedBlockIDsAliases := []string{"Marker-0/1", "0/1-preTSCSeq1_0", "0/1-preTSCSeq1_1", "0/1-preTSCSeq1_2", "0/1-postTSCSeq1_0", "0/1-postTSCSeq1_1", "0/1-postTSCSeq1_2", "0/1-postTSCSeq1_3", "0/1-postTSCSeq1_4", "0/1-postTSCSeq1_5", "Marker-1/2", "0/1-preTSCSeq2_0", "0/1-preTSCSeq2_1", "0/1-preTSCSeq2_2", "0/1-postTSCSeq2_0", "0/1-postTSCSeq2_1", "0/1-postTSCSeq2_2", "0/1-postTSCSeq2_3", "0/1-postTSCSeq2_4", "0/1-postTSCSeq2_5", "Marker-2/2", "2/2_0", "2/2_1", "2/2_2", "2/2_3", "2/2_4", "Marker-2/3"}
	acceptedMarkers := []markers.Marker{markers.NewMarker(0, 1), markers.NewMarker(1, 2), markers.NewMarker(2, 3)}
	tf.SetBlocksAccepted(acceptedBlockIDsAliases...)
	tf.SetMarkersAccepted(acceptedMarkers...)
	tf.SetAcceptedTime(tf.Tangle.BlockDAG.Block("Marker-2/3").IssuingTime())
	require.Eventually(t, tf.Engine.IsBootstrapped, 1*time.Minute, 500*time.Millisecond)

	// case 0 - only one block can attach to genesis, so there should not be two subtangles starting from the genesis, but TSC allows using such tip.
	tf.AssertIsPastConeTimestampCorrect("7/1_2", false)
	// case #1
	tf.AssertIsPastConeTimestampCorrect("0/3_4", false)
	// case #2
	tf.AssertIsPastConeTimestampCorrect("1/3_4", true)
	// case #3
	tf.AssertIsPastConeTimestampCorrect("2/3_4", true)
	// case #4 (marker block)
	tf.AssertIsPastConeTimestampCorrect("Marker-1/2", true)
	// case #5
	tf.AssertIsPastConeTimestampCorrect("2/5_4", false)
	// case #6 (attach to unconfirmed block older than TSC)
	tf.AssertIsPastConeTimestampCorrect("0/1-preTSC_2", false)
	// case #7
	tf.AssertIsPastConeTimestampCorrect("3/2_4", true)
	// case #8
	tf.AssertIsPastConeTimestampCorrect("2/3+0/4_3", false)
	// case #9
	tf.AssertIsPastConeTimestampCorrect("Marker-4/5", false)
	// case #10 (attach to confirmed block older than TSC)
	tf.AssertIsPastConeTimestampCorrect("0/1-preTSCSeq2_2", false)
	// case #11
	tf.AssertIsPastConeTimestampCorrect("5/2_4", false)
	// case #12 (attach to 0/1-postTSCSeq3_4)
	tf.AssertIsPastConeTimestampCorrect("0/1-postTSCSeq3_4", true)
	// case #13 (attach to unconfirmed block younger than TSC, with confirmed past marker older than TSC)
	tf.AssertIsPastConeTimestampCorrect("0/1-postTSC_2", false)
	// case #14
	tf.AssertIsPastConeTimestampCorrect("6/2_4", false)
	// case #15
	tf.AssertIsPastConeTimestampCorrect("0/1-preTSCSeq5_4", false)
	// case #16
	tf.AssertIsPastConeTimestampCorrect("0/1-postTSC-direct_0", false)
}

// Test based on packages/tangle/images/TSC_test_scenario.png.
func TestTipManager_TimeSinceConfirmation_MultipleParents(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	genesisTime := time.Now().Add(-5 * time.Hour)
	tf := NewTestFramework(t,
		workers.CreateGroup("TipManagerTestFramework"),
		WithGenesisUnixTime(genesisTime.Unix()),
		WithTipManagerOptions(
			WithTimeSinceConfirmationThreshold(5*time.Minute),
		),
		WithSlotNotarizationOptions(
			slotnotarization.WithMinCommittableSlotAge(20*6),
		),
		WithBookerOptions(
			markerbooker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](
					markers.WithMaxPastMarkerDistance(10),
				),
			),
		),
		WithEngineOptions(
			engine.WithBootstrapThreshold(time.Since(genesisTime.Add(30*time.Second))),
		),
	)
	tf.Engine.EvictionState.AddRootBlock(models.EmptyBlockID, commitment.ID{})

	createTestTangleMultipleParents(tf)

	acceptedBlockIDsAliases := []string{"Marker-0/1", "Marker-0/2", "Marker-0/3"}
	acceptedMarkers := []markers.Marker{markers.NewMarker(0, 1), markers.NewMarker(0, 2), markers.NewMarker(0, 3)}
	tf.SetBlocksAccepted(acceptedBlockIDsAliases...)
	tf.SetMarkersAccepted(acceptedMarkers...)
	tf.SetAcceptedTime(tf.Tangle.BlockDAG.Block("Marker-0/3").IssuingTime())
	require.Eventually(t, tf.Engine.IsBootstrapped, 1*time.Minute, 500*time.Millisecond)

	// As we advance ATT, Genesis should be beyond TSC, and thus invalid.
	tf.AssertIsPastConeTimestampCorrect("Genesis", false)

	tf.AssertIsPastConeTimestampCorrect("Marker-0/1", false)

	tf.AssertIsPastConeTimestampCorrect("IncorrectTip2", false)

	// case #1
	tf.AssertIsPastConeTimestampCorrect("IncorrectTip", false)
}

func createTestTangleMultipleParents(tf *TestFramework) {
	now := tf.Engine.SlotTimeProvider().GenesisTime().Add(20 * time.Minute)

	// SEQUENCE 0
	{
		tf.Tangle.BlockDAG.CreateBlock("Marker-0/1", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis")), models.WithIssuingTime(now.Add(-9*time.Minute)))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/1")

		tf.Tangle.BlockDAG.CreateBlock("Marker-0/2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Marker-0/1")), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/2")

		tf.Tangle.BlockDAG.CreateBlock("Marker-0/3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Marker-0/2")), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/3")

		tf.Tangle.BlockDAG.CreateBlock("Marker-0/4", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Marker-0/3")), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/4")

		tf.Tangle.BlockDAG.CreateBlock("IncorrectTip", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Marker-0/1", "Marker-0/3")), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("IncorrectTip")

		tf.Tangle.BlockDAG.CreateBlock("IncorrectTip2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Marker-0/1")), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("IncorrectTip2")
	}
}

func createTestTangleTSC(tf *TestFramework) {
	now := tf.Engine.SlotTimeProvider().GenesisTime().Add(20 * time.Minute)

	var lastBlockAlias string

	// SEQUENCE 0
	{
		tf.Tangle.BlockDAG.CreateBlock("Marker-0/1", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis")), models.WithIssuingTime(now.Add(-9*time.Minute)))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/1")
		lastBlockAlias = issueBlocks(tf, "0/1-preTSC", 3, []string{"Marker-0/1"}, time.Minute*8)
		lastBlockAlias = issueBlocks(tf, "0/1-postTSC", 3, []string{lastBlockAlias}, time.Minute)
		tf.Tangle.BlockDAG.CreateBlock("Marker-0/2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/2")
		lastBlockAlias = issueBlocks(tf, "0/2", 5, []string{"Marker-0/2"}, 0)
		tf.Tangle.BlockDAG.CreateBlock("Marker-0/3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/3")
		lastBlockAlias = issueBlocks(tf, "0/3", 5, []string{"Marker-0/3"}, 0)
		tf.Tangle.BlockDAG.CreateBlock("Marker-0/4", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-0/4")
		_ = issueBlocks(tf, "0/4", 5, []string{"Marker-0/4"}, 0)

		// issue block for test case #16
		tf.Tangle.BlockDAG.CreateBlock("0/1-postTSC-direct_0", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Marker-0/1")), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("0/1-postTSC-direct_0")
	}

	// SEQUENCE 1
	{
		lastBlockAlias = issueBlocks(tf, "0/1-preTSCSeq1", 3, []string{"Marker-0/1"}, time.Minute*6)
		lastBlockAlias = issueBlocks(tf, "0/1-postTSCSeq1", 6, []string{lastBlockAlias}, time.Minute*4)
		tf.Tangle.BlockDAG.CreateBlock("Marker-1/2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now.Add(-3*time.Minute)))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-1/2")
		lastBlockAlias = issueBlocks(tf, "1/2", 5, []string{"Marker-1/2"}, 0)
		tf.Tangle.BlockDAG.CreateBlock("Marker-1/3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-1/3")
		_ = issueBlocks(tf, "1/3", 5, []string{"Marker-1/3"}, 0)
	}

	// SEQUENCE 2
	{
		lastBlockAlias = issueBlocks(tf, "0/1-preTSCSeq2", 3, []string{"Marker-0/1"}, time.Minute*6)
		lastBlockAlias = issueBlocks(tf, "0/1-postTSCSeq2", 6, []string{lastBlockAlias}, time.Minute*4)
		tf.Tangle.BlockDAG.CreateBlock("Marker-2/2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now.Add(-3*time.Minute)))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-2/2")
		lastBlockAlias = issueBlocks(tf, "2/2", 5, []string{"Marker-2/2"}, 0)
		tf.Tangle.BlockDAG.CreateBlock("Marker-2/3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-2/3")
		_ = issueBlocks(tf, "2/3", 5, []string{"Marker-2/3"}, 0)
	}

	// SEQUENCE 2 + 0
	{
		tf.Tangle.BlockDAG.CreateBlock("Marker-2/5", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("0/4_4", "2/3_4")), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-2/5")
		_ = issueBlocks(tf, "2/5", 5, []string{"Marker-2/5"}, 0)
	}

	// SEQUENCE 3
	{
		lastBlockAlias = issueBlocks(tf, "0/1-postTSCSeq3", 5, []string{"0/1-postTSCSeq2_0"}, 0)
		tf.Tangle.BlockDAG.CreateBlock("Marker-3/2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-3/2")
		_ = issueBlocks(tf, "3/2", 5, []string{"Marker-3/2"}, 0)
	}

	// SEQUENCE 2 + 0 (two past markers) -> SEQUENCE 4
	{
		lastBlockAlias = issueBlocks(tf, "2/3+0/4", 5, []string{"0/4_4", "2/3_4"}, 0)
		tf.Tangle.BlockDAG.CreateBlock("Marker-4/5", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-4/5")
	}
	// SEQUENCE 5
	{
		lastBlockAlias = issueBlocks(tf, "0/1-preTSCSeq5", 6, []string{"0/1-preTSCSeq2_2"}, time.Minute*6)
		tf.Tangle.BlockDAG.CreateBlock("Marker-5/2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-5/2")
		_ = issueBlocks(tf, "5/2", 5, []string{"Marker-5/2"}, 0)
	}

	// SEQUENCE 6
	{
		lastBlockAlias = issueBlocks(tf, "0/1-postTSCSeq6", 6, []string{"0/1-preTSCSeq2_2"}, 0)
		tf.Tangle.BlockDAG.CreateBlock("Marker-6/2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(lastBlockAlias)), models.WithIssuingTime(now))
		tf.Tangle.BlockDAG.IssueBlocks("Marker-6/2")
		_ = issueBlocks(tf, "6/2", 5, []string{"Marker-6/2"}, 0)
	}

	// SEQUENCE 7 (without markers)
	{
		_ = issueBlocks(tf, "7/1", 3, []string{"Genesis"}, 0)
	}
}

func issueBlocks(tf *TestFramework, blockPrefix string, blockCount int, parents []string, timestampOffset time.Duration) string {
	now := tf.SlotTimeProvider().GenesisTime().Add(20 * time.Minute)

	blockAlias := fmt.Sprintf("%s_%d", blockPrefix, 0)

	tf.Tangle.BlockDAG.CreateBlock(blockAlias, models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(parents...)), models.WithIssuingTime(now.Add(-timestampOffset)))
	tf.Tangle.BlockDAG.IssueBlocks(blockAlias)

	for i := 1; i < blockCount; i++ {
		alias := fmt.Sprintf("%s_%d", blockPrefix, i)
		tf.Tangle.BlockDAG.CreateBlock(alias, models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs(blockAlias)), models.WithIssuingTime(now.Add(-timestampOffset)))
		tf.Tangle.BlockDAG.IssueBlocks(alias)
		// fmt.Println("issuing block", tf.Tangle.BlockDAG.Block(alias).ID())
		blockAlias = alias
	}
	return blockAlias
}

func TestTipManager_TimeSinceConfirmation_RootBlockParent(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	genesisTime := time.Now().Add(-5 * time.Hour)
	tf := NewTestFramework(t,
		workers.CreateGroup("TipManagerTestFramework"),
		WithGenesisUnixTime(genesisTime.Unix()),
		WithBookerOptions(
			markerbooker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](
					markers.WithMaxPastMarkerDistance(10),
				),
			),
		),
		WithEngineOptions(
			engine.WithBootstrapThreshold(time.Since(genesisTime.Add(-time.Hour))),
		),
		WithTipManagerOptions(
			WithTimeSinceConfirmationThreshold(30*time.Second),
		),
	)

	now := tf.SlotTimeProvider().GenesisTime().Add(5 * time.Minute)

	tf.Engine.EvictionState.AddRootBlock(models.EmptyBlockID, commitment.ID{})

	tf.Tangle.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis")), models.WithIssuingTime(now.Add(-50*time.Second)))
	tf.Tangle.BlockDAG.IssueBlocks("Block1")
	tf.Tangle.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1")), models.WithIssuingTime(now))
	tf.Tangle.BlockDAG.IssueBlocks("Block2")
	tf.Tangle.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block2")), models.WithIssuingTime(now.Add(5*time.Second)))
	tf.Tangle.BlockDAG.IssueBlocks("Block3")
	tf.Tangle.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block3")), models.WithIssuingTime(now.Add(10*time.Second)))
	tf.Tangle.BlockDAG.IssueBlocks("Block4")

	acceptedBlockIDsAliases := []string{"Block1", "Block2"}
	acceptedMarkers := []markers.Marker{markers.NewMarker(0, 1), markers.NewMarker(0, 2)}
	tf.SetBlocksAccepted(acceptedBlockIDsAliases...)
	tf.SetMarkersAccepted(acceptedMarkers...)
	tf.SetAcceptedTime(now.Add(25 * time.Second))
	block := tf.Tangle.BlockDAG.Block("Block1")
	tf.Engine.EvictionState.AddRootBlock(block.ID(), block.Commitment().ID())

	tf.Engine.EvictionState.RemoveRootBlock(models.EmptyBlockID)

	require.Eventually(t, tf.Engine.Notarization.IsFullyCommitted, 1*time.Minute, 500*time.Millisecond)

	tf.Engine.Workers.WaitParents()

	tf.Tangle.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1")), models.WithIssuingTime(now))
	tf.Tangle.BlockDAG.IssueBlocks("Block5")

	tf.AssertIsPastConeTimestampCorrect("Block3", true)
	tf.AssertIsPastConeTimestampCorrect("Block2", true)
	tf.AssertIsPastConeTimestampCorrect("Block1", false)
	tf.AssertIsPastConeTimestampCorrect("Block5", false)
}

func TestTipManager_FutureTips(t *testing.T) {
	t.Skip("Test should be moved to BlockDAG to test the future commitment buffering functionality.")
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	// MinimumCommittableAge will also be 10 seconds
	workers := workerpool.NewGroup(t.Name())
	tf := NewTestFramework(t,
		workers.CreateGroup("TipManagerTestFramework"),
		WithGenesisUnixTime(time.Now().Add(-100*time.Second).Unix()),
		WithSlotNotarizationOptions(slotnotarization.WithMinCommittableSlotAge(1)),
	)
	tf.Engine.EvictionState.AddRootBlock(models.EmptyBlockID, commitment.ID{})

	tf.Engine.Events.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
		fmt.Println(">>", details.Commitment.ID())
	})

	// Let's add a few blocks to slot 1
	{
		blockTime := tf.SlotTimeProvider().GenesisTime().Add(5 * time.Second)
		tf.Tangle.BlockDAG.CreateBlock("Block1.1", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Genesis")), models.WithIssuingTime(blockTime))
		tf.Tangle.BlockDAG.CreateBlock("Block1.2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1.1")), models.WithIssuingTime(blockTime))
		tf.Tangle.BlockDAG.CreateBlock("Block1.3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1.2")), models.WithIssuingTime(blockTime))
		tf.Tangle.BlockDAG.CreateBlock("Block1.4", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1.2", "Block1.3")), models.WithIssuingTime(blockTime))

		tf.Tangle.BlockDAG.IssueBlocks("Block1.1", "Block1.2", "Block1.3", "Block1.4")
		workers.WaitChildren()
		tf.SetBlocksAccepted("Block1.1", "Block1.2", "Block1.3", "Block1.4")
		tf.SetAcceptedTime(tf.Tangle.BlockDAG.Block("Block1.4").IssuingTime())
		workers.WaitChildren()

		tf.AssertTipsAdded(4)
		tf.AssertTipsRemoved(3)
		tf.AssertTips(tf.Tangle.BlockDAG.BlockIDs("Block1.4"))
	}

	// Let's add a few blocks to slot 2
	{
		blockTime := tf.SlotTimeProvider().GenesisTime().Add(15 * time.Second)
		tf.Tangle.BlockDAG.CreateBlock("Block2.1", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block1.4")), models.WithIssuingTime(blockTime))

		tf.Tangle.BlockDAG.IssueBlocks("Block2.1")
		workers.WaitChildren()
		tf.SetBlocksAccepted("Block2.1")
		tf.SetAcceptedTime(tf.Tangle.BlockDAG.Block("Block2.1").IssuingTime())
		workers.WaitChildren()

		tf.AssertTipsAdded(5)
		tf.AssertTipsRemoved(4)
		tf.AssertTips(tf.Tangle.BlockDAG.BlockIDs("Block2.1"))
	}

	// Let's add a few blocks to slot 3
	{
		blockTime := tf.SlotTimeProvider().GenesisTime().Add(25 * time.Second)
		tf.Tangle.BlockDAG.CreateBlock("Block3.1", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block2.1")), models.WithIssuingTime(blockTime))

		tf.Tangle.BlockDAG.IssueBlocks("Block3.1")
		workers.WaitChildren()
		tf.SetBlocksAccepted("Block3.1")
		tf.SetAcceptedTime(tf.Tangle.BlockDAG.Block("Block3.1").IssuingTime())
		workers.WaitChildren()

		tf.AssertTipsAdded(6)
		tf.AssertTipsRemoved(5)
		tf.AssertTips(tf.Tangle.BlockDAG.BlockIDs("Block3.1"))
	}

	commitment2_1 := tf.FormCommitment(2, []string{}, 1)
	commitment2_2 := tf.FormCommitment(2, []string{"Block2.1"}, 1)
	commitment3_1 := commitment.New(3, commitment2_1.ID(), types.Identifier{1}, 0)

	// Let's introduce a future tip, in slot 4
	{
		blockTime := tf.SlotTimeProvider().GenesisTime().Add(35 * time.Second)
		tf.Tangle.BlockDAG.CreateBlock("Block4.1", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block3.1")), models.WithIssuingTime(blockTime), models.WithCommitment(commitment2_1))
		tf.Tangle.BlockDAG.CreateBlock("Block4.2", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block3.1")), models.WithIssuingTime(blockTime), models.WithCommitment(commitment2_2))
		tf.Tangle.BlockDAG.CreateBlock("Block4.3", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block3.1")), models.WithIssuingTime(blockTime), models.WithCommitment(commitment3_1))
		tf.Tangle.BlockDAG.CreateBlock("Block4.4", models.WithStrongParents(tf.Tangle.BlockDAG.BlockIDs("Block4.2")), models.WithIssuingTime(blockTime), models.WithCommitment(commitment2_2))

		tf.Tangle.BlockDAG.IssueBlocks("Block4.1", "Block4.2", "Block4.3", "Block4.4")
		workers.WaitChildren()

		tf.AssertTipsAdded(6)
		tf.AssertTipsRemoved(5)
		tf.AssertTips(tf.Tangle.BlockDAG.BlockIDs("Block3.1"))
		// tf.AssertFutureTips(map[slot.Index]map[commitment.ID]models.BlockIDs{
		//	2: {
		//		commitment2_1.ID(): tf.Tangle.BlockDAG.BlockIDs("Block4.1"),
		//		commitment2_2.ID(): tf.Tangle.BlockDAG.BlockIDs("Block4.2", "Block4.4"),
		//	},
		//	3: {
		//		commitment3_1.ID(): tf.Tangle.BlockDAG.BlockIDs("Block4.3"),
		//	},
		// })
	}

	// We accept a block of slot 4, rendering slot 2 committable and refreshing the tippool
	{
		tf.SetBlocksAccepted("Block4.2")
		tf.SetAcceptedTime(tf.Tangle.BlockDAG.Block("Block4.2").IssuingTime())
		workers.WaitChildren()

		// tf.AssertFutureTips(map[slot.Index]map[commitment.ID]models.BlockIDs{
		//	3: {
		//		commitment3_1.ID(): tf.Tangle.BlockDAG.BlockIDs("Block4.3"),
		//	},
		// })

		tf.AssertTipsAdded(7)
		tf.AssertTipsRemoved(6)
		tf.AssertTips(tf.Tangle.BlockDAG.BlockIDs("Block4.4"))

		tf.AssertEqualBlocks(tf.Instance.Tips(1), tf.Tangle.BlockDAG.BlockIDs("Block4.4"))
	}
}
