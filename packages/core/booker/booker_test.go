package booker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

func Test(t *testing.T) {
	tf := NewTestFramework(t)
	// defer tf.Shutdown()
	tf.ledgerTf.CreateTransaction("tx1", 4, "Genesis")
	tf.ledgerTf.CreateTransaction("tx2", 4, "tx1.0")
	tf.ledgerTf.CreateTransaction("tx3", 4, "tx2.2")
	tf.ledgerTf.CreateTransaction("tx4", 4, "tx3.1")

	tf.CreateBlock("block1", models.WithPayload(tf.ledgerTf.Transaction("tx4")))
	tf.CreateBlock("block2", models.WithPayload(tf.ledgerTf.Transaction("tx4")))

	tf.CreateBlock("block3", models.WithPayload(tf.ledgerTf.Transaction("tx3")))

	tf.CreateBlock("block4", models.WithPayload(tf.ledgerTf.Transaction("tx2")))

	tf.CreateBlock("block5", models.WithPayload(tf.ledgerTf.Transaction("tx1")))

	tf.IssueBlocks("block1").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block3").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block5").WaitUntilAllTasksProcessed()

	fmt.Println(tf.Booker.Block(tf.Block("block1").ID()))
}

func TestBooker_Marker(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))
	tf.CreateBlock("block3", models.WithStrongParents(tf.BlockIDs("block1")))
	tf.CreateBlock("block4", models.WithStrongParents(tf.BlockIDs("block3")))
	tf.CreateBlock("block5", models.WithStrongParents(tf.BlockIDs("block4")))
	tf.CreateBlock("block6", models.WithStrongParents(tf.BlockIDs("block5")))

	tf.IssueBlocks("block1", "block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block3", "block4", "block5", "block6").WaitUntilAllTasksProcessed()
}

func Test_Conflict(t *testing.T) {
	tf := NewTestFramework(t)
	// defer tf.Shutdown()
	tf.ledgerTf.CreateTransaction("tx1", 4, "Genesis")
	tf.ledgerTf.CreateTransaction("tx2", 4, "tx1.0")
	tf.ledgerTf.CreateTransaction("tx3", 4, "tx1.0")

	tf.CreateBlock("block1", models.WithPayload(tf.ledgerTf.Transaction("tx1")))

	tf.CreateBlock("block2", models.WithPayload(tf.ledgerTf.Transaction("tx2")))

	tf.CreateBlock("block3", models.WithPayload(tf.ledgerTf.Transaction("tx3")))

	tf.IssueBlocks("block1").WaitUntilAllTasksProcessed()
	//tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	//tf.IssueBlocks("block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block3").WaitUntilAllTasksProcessed()

	blk, _ := tf.Booker.Block(tf.Block("block3").ID())
	fmt.Println(tf.Booker.blockBookingDetails(blk))
}

/////////////////////////////

func TestScenario_1(t *testing.T) {
	debug.SetEnabled(true)

	//tangle := NewTestTangle(WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	tf := NewTestFramework(t)
	defer tf.Shutdown()

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX1", 3, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.ledgerTf.Transaction("TX2")))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX3", 1, "TX1.0")))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX4", 1, "TX1.0")))
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block2", "Block5")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX5", 1, "TX2.0", "TX4.0")))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block4", "Block5")), models.WithPayload(tf.ledgerTf.Transaction("TX2")))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block4", "Block5")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX6", 1, "TX3.0", "TX4.0")))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block4", "Block6")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX7", 1, "TX5.0")))

	tf.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5", "Block6", "Block7", "Block8", "Block9").WaitUntilAllTasksProcessed()

	for _, alias := range []string{"Block1", "Block2", "Block3", "Block4", "Block5", "Block6", "Block7", "Block8", "Block9"} {
		block := tf.Block(alias)
		_, blockConflictIDs := tf.Booker.blockBookingDetails(block)
		fmt.Println(alias, blockConflictIDs)

		fmt.Println(alias, "added", block.AddedConflictIDs(), "subtracted", block.SubtractedConflictIDs())
		fmt.Println(alias, "all", block.StructureDetails())
		fmt.Println("UTXO", tf.Booker.PayloadConflictIDs(block))
		fmt.Println("-----------------------------------------------------")
	}

	tf.checkConflictIDs(map[string]*set.AdvancedSet[utxo.TransactionID]{
		"Block1": utxo.NewTransactionIDs(),
		"Block3": utxo.NewTransactionIDs(),
		"Block2": utxo.NewTransactionIDs(),
		"Block4": tf.ledgerTf.TransactionIDs("TX3"),
		"Block5": tf.ledgerTf.TransactionIDs("TX4"),
		"Block6": tf.ledgerTf.TransactionIDs("TX4", "TX5"),
		"Block7": tf.ledgerTf.TransactionIDs("TX4", "TX3"),
		"Block8": tf.ledgerTf.TransactionIDs("TX4", "TX3", "TX6"),
		"Block9": tf.ledgerTf.TransactionIDs("TX4", "TX3", "TX5"),
	})
}

//func checkMarkers(t *testing.T, testFramework *TestFramework, expectedMarkers map[string]*markersold.Markers) {
//	for blockID, expectedMarkersOfBlock := range expectedMarkers {
//		block := testFramework.Block(blockID)
//		assert.True(t, expectedMarkersOfBlock.Equals(block.StructureDetails().PastMarkers()), "Markers of %s are wrong.\n"+
//			"Expected: %+v\nActual: %+v", blockID, expectedMarkersOfBlock, blockMetadata.StructureDetails().PastMarkers)
//
//		// if we have only a single marker - check if the marker is mapped to this block (or its inherited past marker)
//		if expectedMarkersOfBlock.Size() == 1 {
//			currentMarker := expectedMarkersOfBlock.Marker()
//
//			mappedBlockIDOfMarker := testFramework.tangle.Booker.MarkersManager.BlockID(currentMarker)
//			currentBlockID := testFramework.Block(blockID).ID()
//
//			if mappedBlockIDOfMarker == currentBlockID {
//				continue
//			}
//
//			assert.True(t, testFramework.tangle.Storage.BlockMetadata(mappedBlockIDOfMarker).Consume(func(blockMetadata *BlockMetadata) {
//				// Blocks attaching to Genesis can have 0,0 as a PastMarker, so do not check Markers -> Block.
//				if currentMarker.SequenceID() == 0 && currentMarker.Index() == 0 {
//					return
//				}
//
//				if assert.True(t, blockMetadata.StructureDetails().IsPastMarker(), "Block with %s should be PastMarker", blockMetadata.ID()) {
//					assert.True(t, blockMetadata.StructureDetails().PastMarkers().Marker() == currentMarker, "PastMarker of %s is wrong.\n"+
//						"Expected: %+v\nActual: %+v", blockMetadata.ID(), currentMarker, blockMetadata.StructureDetails().PastMarkers().Marker())
//				}
//			}), "failed to load Block with %s", mappedBlockIDOfMarker)
//		}
//	}
//}

//func checkNormalizedConflictIDsContained(t *testing.T, testFramework *TestFramework, expectedContainedConflictIDs map[string]*set.AdvancedSet[utxo.TransactionID]) {
//	for blockID, blockExpectedConflictIDs := range expectedContainedConflictIDs {
//		retrievedConflictIDs, errRetrieve := testFramework.tangle.Booker.BlockConflictIDs(testFramework.Block(blockID).ID())
//		assert.NoError(t, errRetrieve)
//
//		normalizedRetrievedConflictIDs := retrievedConflictIDs.Clone()
//		for it := retrievedConflictIDs.Iterator(); it.HasNext(); {
//			testFramework.tangle.Ledger.ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
//				normalizedRetrievedConflictIDs.DeleteAll(b.Parents())
//			})
//		}
//
//		normalizedExpectedConflictIDs := blockExpectedConflictIDs.Clone()
//		for it := blockExpectedConflictIDs.Iterator(); it.HasNext(); {
//			testFramework.tangle.Ledger.ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
//				normalizedExpectedConflictIDs.DeleteAll(b.Parents())
//			})
//		}
//
//		assert.True(t, normalizedExpectedConflictIDs.Intersect(normalizedRetrievedConflictIDs).Size() == normalizedExpectedConflictIDs.Size(), "ConflictID of %s should be %s but is %s", blockID, normalizedExpectedConflictIDs, normalizedRetrievedConflictIDs)
//	}
//}
//
//func checkBlockMetadataDiffConflictIDs(t *testing.T, testFramework *TestFramework, expectedDiffConflictIDs map[string][]*set.AdvancedSet[utxo.TransactionID]) {
//	for blockID, expectedDiffConflictID := range expectedDiffConflictIDs {
//		assert.True(t, testFramework.tangle.Storage.BlockMetadata(testFramework.Block(blockID).ID()).Consume(func(blockMetadata *BlockMetadata) {
//			assert.True(t, expectedDiffConflictID[0].Equal(blockMetadata.AddedConflictIDs()), "AddConflictIDs of %s should be %s but is %s in the Metadata", blockID, expectedDiffConflictID[0], blockMetadata.AddedConflictIDs())
//		}))
//		assert.True(t, testFramework.tangle.Storage.BlockMetadata(testFramework.Block(blockID).ID()).Consume(func(blockMetadata *BlockMetadata) {
//			assert.True(t, expectedDiffConflictID[1].Equal(blockMetadata.SubtractedConflictIDs()), "SubtractedConflictIDs of %s should be %s but is %s in the Metadata", blockID, expectedDiffConflictID[1], blockMetadata.SubtractedConflictIDs())
//		}))
//	}
//}
