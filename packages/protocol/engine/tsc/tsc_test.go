package tsc

import (
	"container/heap"
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

func TestOrphanageManager_removeElementFromHeap(t *testing.T) {
	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))

	now := time.Now()
	blocks := make([]*blockdag.Block, 0, 20)
	for i := 0; i < 20; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		block := blockdag.NewBlock(tf.CreateBlock(alias, models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuingTime(now.Add(time.Duration(i)*time.Second))), blockdag.WithSolid(true))
		blocks = append(blocks, block)
		heap.Push(&tf.OrphanageManager.unconfirmedBlocks, &QueueElement{Key: block.IssuingTime(), Value: block})
	}
	tf.OrphanageManager.removeElementFromHeap(blocks[10])
	assert.Len(t, tf.OrphanageManager.unconfirmedBlocks, 19)
	tf.OrphanageManager.removeElementFromHeap(blocks[10])
	assert.Len(t, tf.OrphanageManager.unconfirmedBlocks, 19)
}

func TestOrphanageManager_orphanBeforeTSC(t *testing.T) {
	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))

	now := time.Now()
	blocks := make([]*blockdag.Block, 0, 20)
	for i := 0; i < 20; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		block := blockdag.NewBlock(tf.CreateBlock(alias, models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuingTime(now.Add(time.Duration(i)*time.Second))), blockdag.WithSolid(true))
		blocks = append(blocks, block)
		heap.Push(&tf.OrphanageManager.unconfirmedBlocks, &QueueElement{Key: block.IssuingTime(), Value: block})
	}

	tf.OrphanageManager.orphanBeforeTSC(now.Add(time.Duration(10) * time.Second))
	tf.AssertOrphanedCount(10, "%d blocks should be orphaned", 10)
}

func TestOrphanageManager_HandleTimeUpdate(t *testing.T) {
	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))

	createTestTangleOrphanage(tf)

	lo.MergeMaps(tf.mockAcceptance.acceptedBlocks, map[models.BlockID]bool{
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

	for blockID, accepted := range tf.mockAcceptance.acceptedBlocks {
		if !accepted {
			continue
		}
		virtualVotingBlock, _ := tf.VirtualVoting.Block(blockID)
		//tf.mockAcceptance.blockAcceptedEvent.Trigger(acceptance.NewBlock(virtualVotingBlock, acceptance.WithAccepted(true)))
		tf.optsClock.SetAcceptedTime(virtualVotingBlock.IssuingTime())
	}

	event.Loop.WaitUntilAllTasksProcessed()

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
}

//
//func TestOrphanageManager_HandleAcceptedBlock(t *testing.T) {
//	tf := NewTestFramework(t, WithTSCManagerOptions(WithTimeSinceConfirmationThreshold(30*time.Second)))
//
//	createTestTangleOrphanage(tf)
//	confirmedBlockIDsString := []string{"Marker-0/1", "0/1-preTSC_0", "0/1-preTSC_1", "0/1-preTSC_2", "0/1-preTSC_3", "0/1-preTSC_4", "0/1-preTSC_5", "0/1-preTSC_6", "0/1-preTSC_7", "0/1-preTSC_8", "0/1-preTSC_9", "0/1-postTSC_0"}
//	confirmedBlockIDs := prepareConfirmedBlockIDs(testFramework, confirmedBlockIDsString)
//
//	confirmationOracle.Lock()
//	confirmationOracle.confirmedBlockIDs = confirmedBlockIDs
//	confirmationOracle.Unlock()
//	assert.Equal(t, 27, orphanManager.unconfirmedBlocks.Len())
//	assert.Equal(t, 25, len(orphanManager.strongChildCounters))
//
//	for _, blockID := range confirmedBlockIDsString {
//		confirmationOracle.Events().BlockAccepted.Trigger(&BlockAcceptedEvent{Block: testFramework.Block(blockID)})
//	}
//	event.Loop.WaitUntilAllTasksProcessed()
//
//	// should it also remove the block from the heap?
//	assert.Equal(t, int32(15), orphanedBlocks.Load())
//	assert.Equal(t, 0, orphanManager.unconfirmedBlocks.Len())
//	assert.Equal(t, 0, len(orphanManager.strongChildCounters))
//}

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
