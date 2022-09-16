package tsc

import (
	"container/heap"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generalheap"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/timed"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/blockdag"
	models2 "github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/models"
)

func TestOrphanageManager_orphanBeforeTSC(t *testing.T) {
	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))

	now := time.Now()
	blocks := make([]*blockdag.Block, 0, 20)
	for i := 0; i < 20; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		block := blockdag.NewBlock(tf.CreateBlock(alias, models2.WithStrongParents(tf.BlockIDs("Genesis")), models2.WithIssuingTime(now.Add(time.Duration(i)*time.Second))), blockdag.WithSolid(true))
		blocks = append(blocks, block)
		heap.Push(&tf.OrphanageManager.unconfirmedBlocks, &generalheap.HeapElement[timed.HeapKey, *blockdag.Block]{Key: timed.HeapKey(block.IssuingTime()), Value: block})
	}

	tf.OrphanageManager.orphanBeforeTSC(now.Add(time.Duration(10) * time.Second))
	tf.AssertOrphanedCount(11, "%d blocks should be orphaned", 1)
}

func TestOrphanageManager_HandleTimeUpdate(t *testing.T) {
	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))

	createTestTangleOrphanage(tf)

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, map[models2.BlockID]bool{
		tf.Block("Marker-0/1").ID():    true,
		tf.Block("0/1-preTSC_0").ID():  true,
		tf.Block("0/1-preTSC_1").ID():  true,
		tf.Block("0/1-preTSC_2").ID():  true,
		tf.Block("0/1-preTSC_3").ID():  true,
		tf.Block("0/1-preTSC_4").ID():  true,
		tf.Block("0/1-preTSC_5").ID():  true,
		tf.Block("0/1-preTSC_6").ID():  true,
		tf.Block("0/1-preTSC_7").ID():  true,
		tf.Block("0/1-preTSC_8").ID():  true,
		tf.Block("0/1-preTSC_9").ID():  true,
		tf.Block("0/1-postTSC_0").ID(): true,
	})

	assert.Equal(t, 27, tf.OrphanageManager.unconfirmedBlocks.Len())

	for blockID, accepted := range tf.mockAcceptance.AcceptedBlocks {
		if !accepted {
			continue
		}
		virtualVotingBlock, _ := tf.VirtualVoting.Block(blockID)
		tf.optsClock.SetAcceptedTime(virtualVotingBlock.IssuingTime())
	}

	tf.BlockDAGTestFramework.WaitUntilAllTasksProcessed()

	tf.AssertOrphanedCount(15, "expected %d orphaned blocks", 15)
	tf.AssertExplicitlyOrphaned(map[string]bool{
		"0/1-preTSCSeq1_0":  true,
		"0/1-preTSCSeq1_1":  true,
		"0/1-preTSCSeq1_2":  true,
		"0/1-preTSCSeq1_3":  true,
		"0/1-preTSCSeq1_4":  true,
		"0/1-preTSCSeq1_5":  true,
		"0/1-preTSCSeq1_6":  true,
		"0/1-preTSCSeq1_7":  true,
		"0/1-preTSCSeq1_8":  true,
		"0/1-preTSCSeq1_9":  true,
		"0/1-postTSCSeq1_0": false,
		"0/1-postTSCSeq1_1": false,
		"0/1-postTSCSeq1_2": false,
		"0/1-postTSCSeq1_3": false,
		"0/1-postTSCSeq1_4": false,
	})
	assert.Equal(t, 6, tf.OrphanageManager.unconfirmedBlocks.Len())

	// Mark orphaned blocks as accepted and make sure that they get unorphaned.
	{
		newAcceptedBlocksInOrder := []models2.BlockID{
			tf.Block("0/1-preTSCSeq1_0").ID(),
			tf.Block("0/1-preTSCSeq1_1").ID(),
			tf.Block("0/1-preTSCSeq1_2").ID(),
			tf.Block("0/1-preTSCSeq1_3").ID(),
			tf.Block("0/1-preTSCSeq1_4").ID(),
			tf.Block("0/1-preTSCSeq1_5").ID(),
			tf.Block("0/1-preTSCSeq1_6").ID(),
			tf.Block("0/1-preTSCSeq1_7").ID(),
			tf.Block("0/1-preTSCSeq1_8").ID(),
			tf.Block("0/1-preTSCSeq1_9").ID(),
			tf.Block("0/1-postTSCSeq1_0").ID(),
			tf.Block("0/1-postTSCSeq1_1").ID(),
			tf.Block("0/1-postTSCSeq1_2").ID(),
			tf.Block("0/1-postTSCSeq1_3").ID(),
			tf.Block("0/1-postTSCSeq1_4").ID(),
		}
		newAcceptedBlocks := map[models2.BlockID]bool{
			tf.Block("0/1-preTSCSeq1_0").ID():  true,
			tf.Block("0/1-preTSCSeq1_1").ID():  true,
			tf.Block("0/1-preTSCSeq1_2").ID():  true,
			tf.Block("0/1-preTSCSeq1_3").ID():  true,
			tf.Block("0/1-preTSCSeq1_4").ID():  true,
			tf.Block("0/1-preTSCSeq1_5").ID():  true,
			tf.Block("0/1-preTSCSeq1_6").ID():  true,
			tf.Block("0/1-preTSCSeq1_7").ID():  true,
			tf.Block("0/1-preTSCSeq1_8").ID():  true,
			tf.Block("0/1-preTSCSeq1_9").ID():  true,
			tf.Block("0/1-postTSCSeq1_0").ID(): true,
			tf.Block("0/1-postTSCSeq1_1").ID(): true,
			tf.Block("0/1-postTSCSeq1_2").ID(): true,
			tf.Block("0/1-postTSCSeq1_3").ID(): true,
			tf.Block("0/1-postTSCSeq1_4").ID(): true,
		}
		lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, newAcceptedBlocks)

		for _, blockID := range newAcceptedBlocksInOrder {
			virtualVotingBlock, _ := tf.VirtualVoting.Block(blockID)
			tf.optsClock.SetAcceptedTime(virtualVotingBlock.IssuingTime())
			tf.BlockDAG.SetOrphaned(virtualVotingBlock.Block.Block, false)
		}

		tf.BlockDAGTestFramework.WaitUntilAllTasksProcessed()

		tf.AssertOrphanedCount(0, "expected %d orphaned blocks", 0)
		tf.AssertExplicitlyOrphaned(map[string]bool{
			"0/1-preTSCSeq1_0":  false,
			"0/1-preTSCSeq1_1":  false,
			"0/1-preTSCSeq1_2":  false,
			"0/1-preTSCSeq1_3":  false,
			"0/1-preTSCSeq1_4":  false,
			"0/1-preTSCSeq1_5":  false,
			"0/1-preTSCSeq1_6":  false,
			"0/1-preTSCSeq1_7":  false,
			"0/1-preTSCSeq1_8":  false,
			"0/1-preTSCSeq1_9":  false,
			"0/1-postTSCSeq1_0": false,
			"0/1-postTSCSeq1_1": false,
			"0/1-postTSCSeq1_2": false,
			"0/1-postTSCSeq1_3": false,
			"0/1-postTSCSeq1_4": false,
		})
		assert.Equal(t, 6, tf.OrphanageManager.unconfirmedBlocks.Len())
	}
}

func createTestTangleOrphanage(tf *TestFramework) {
	var lastMsgAlias string
	// SEQUENCE 0
	{
		tf.CreateBlock("Marker-0/1", models2.WithStrongParents(tf.BlockIDs("Genesis")), models2.WithIssuingTime(time.Now().Add(-6*time.Minute)))
		tf.IssueBlocks("Marker-0/1").WaitUntilAllTasksProcessed()
		lastMsgAlias = issueBlocks(tf, "0/1-preTSC", 10, []string{"Marker-0/1"}, time.Minute*6)
		lastMsgAlias = issueBlocks(tf, "0/1-postTSC", 1, []string{lastMsgAlias}, 0)

	}

	// SEQUENCE 1
	{ //nolint:dupl
		lastMsgAlias = issueBlocks(tf, "0/1-preTSCSeq1", 10, []string{"Marker-0/1"}, time.Minute*6)
		lastMsgAlias = issueBlocks(tf, "0/1-postTSCSeq1", 5, []string{lastMsgAlias}, time.Second*15)
	}
}

func issueBlocks(tf *TestFramework, blkPrefix string, blkCount int, parents []string, timestampOffset time.Duration) string {
	blkAlias := fmt.Sprintf("%s_%d", blkPrefix, 0)

	tf.CreateBlock(blkAlias, models2.WithStrongParents(tf.BlockIDs(parents...)), models2.WithIssuingTime(time.Now().Add(-timestampOffset)))
	tf.IssueBlocks(blkAlias).WaitUntilAllTasksProcessed()

	for i := 1; i < blkCount; i++ {
		alias := fmt.Sprintf("%s_%d", blkPrefix, i)
		tf.CreateBlock(alias, models2.WithIssuer(identity.GenerateIdentity().PublicKey()), models2.WithStrongParents(tf.BlockIDs(blkAlias)), models2.WithIssuingTime(time.Now().Add(-timestampOffset)))
		tf.IssueBlocks(alias).WaitUntilAllTasksProcessed()
		blkAlias = alias
	}
	return blkAlias
}
