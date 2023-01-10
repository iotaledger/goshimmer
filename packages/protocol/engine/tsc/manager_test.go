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
	"github.com/iotaledger/hive.go/core/types"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestOrphanageManager_orphanBeforeTSC(t *testing.T) {
	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))

	now := time.Now()
	blocks := make([]*blockdag.Block, 0, 20)
	for i := 0; i < 20; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		block := blockdag.NewBlock(tf.CreateBlock(alias, models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuingTime(now.Add(time.Duration(i)*time.Second))), blockdag.WithSolid(true))
		blocks = append(blocks, block)
		heap.Push(&tf.Manager.unacceptedBlocks, &generalheap.HeapElement[timed.HeapKey, *blockdag.Block]{Key: timed.HeapKey(block.IssuingTime()), Value: block})
	}

	tf.Manager.orphanBeforeTSC(now.Add(time.Duration(10) * time.Second))
	tf.AssertOrphanedCount(11, "%d blocks should be orphaned", 1)
}

func TestOrphanageManager_HandleTimeUpdate(t *testing.T) {
	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))

	createTestTangleOrphanage(tf)

	lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, models.BlockIDs{
		tf.Block("Marker-0/1").ID():    types.Void,
		tf.Block("0/1-preTSC_0").ID():  types.Void,
		tf.Block("0/1-preTSC_1").ID():  types.Void,
		tf.Block("0/1-preTSC_2").ID():  types.Void,
		tf.Block("0/1-preTSC_3").ID():  types.Void,
		tf.Block("0/1-preTSC_4").ID():  types.Void,
		tf.Block("0/1-preTSC_5").ID():  types.Void,
		tf.Block("0/1-preTSC_6").ID():  types.Void,
		tf.Block("0/1-preTSC_7").ID():  types.Void,
		tf.Block("0/1-preTSC_8").ID():  types.Void,
		tf.Block("0/1-preTSC_9").ID():  types.Void,
		tf.Block("0/1-postTSC_0").ID(): types.Void,
	})

	assert.Equal(t, 27, tf.Manager.unacceptedBlocks.Len())

	for blockID := range tf.mockAcceptance.AcceptedBlocks {
		virtualVotingBlock, _ := tf.VirtualVoting.Block(blockID)
		tf.Manager.HandleTimeUpdate(virtualVotingBlock.IssuingTime())
	}

	tf.BlockDAGTestFramework.WaitUntilAllTasksProcessed()

	tf.AssertOrphanedCount(10, "expected %d orphaned blocks", 10)
	tf.AssertOrphaned(map[string]bool{
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
	assert.Equal(t, 6, tf.Manager.unacceptedBlocks.Len())

	// Mark orphaned blocks as accepted and make sure that they get unorphaned.
	{
		newAcceptedBlocksInOrder := []models.BlockID{
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
		newAcceptedBlocks := models.BlockIDs{
			tf.Block("0/1-preTSCSeq1_0").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_1").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_2").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_3").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_4").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_5").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_6").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_7").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_8").ID():  types.Void,
			tf.Block("0/1-preTSCSeq1_9").ID():  types.Void,
			tf.Block("0/1-postTSCSeq1_0").ID(): types.Void,
			tf.Block("0/1-postTSCSeq1_1").ID(): types.Void,
			tf.Block("0/1-postTSCSeq1_2").ID(): types.Void,
			tf.Block("0/1-postTSCSeq1_3").ID(): types.Void,
			tf.Block("0/1-postTSCSeq1_4").ID(): types.Void,
		}
		lo.MergeMaps(tf.mockAcceptance.AcceptedBlocks, newAcceptedBlocks)

		for _, blockID := range newAcceptedBlocksInOrder {
			virtualVotingBlock, _ := tf.VirtualVoting.Block(blockID)
			tf.Manager.HandleTimeUpdate(virtualVotingBlock.IssuingTime())
			tf.BlockDAG.SetOrphaned(virtualVotingBlock.Block.Block, false)
		}

		tf.BlockDAGTestFramework.WaitUntilAllTasksProcessed()

		tf.AssertOrphanedCount(0, "expected %d orphaned blocks", 0)
		tf.AssertOrphaned(map[string]bool{
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
		assert.Equal(t, 6, tf.Manager.unacceptedBlocks.Len())
	}
}

func createTestTangleOrphanage(tf *TestFramework) {
	var lastMsgAlias string
	// SEQUENCE 0
	{
		tf.CreateBlock("Marker-0/1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuingTime(time.Now().Add(-6*time.Minute)))
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

	tf.CreateBlock(blkAlias, models.WithStrongParents(tf.BlockIDs(parents...)), models.WithIssuingTime(time.Now().Add(-timestampOffset)))
	tf.IssueBlocks(blkAlias).WaitUntilAllTasksProcessed()

	for i := 1; i < blkCount; i++ {
		alias := fmt.Sprintf("%s_%d", blkPrefix, i)
		tf.CreateBlock(alias, models.WithIssuer(identity.GenerateIdentity().PublicKey()), models.WithStrongParents(tf.BlockIDs(blkAlias)), models.WithIssuingTime(time.Now().Add(-timestampOffset)))
		tf.IssueBlocks(alias).WaitUntilAllTasksProcessed()
		blkAlias = alias
	}
	return blkAlias
}
