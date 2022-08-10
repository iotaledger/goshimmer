package booker

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
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
	// tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
	// tf.IssueBlocks("block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("block3").WaitUntilAllTasksProcessed()

	blk, _ := tf.Booker.Block(tf.Block("block3").ID())
	fmt.Println(tf.Booker.blockBookingDetails(blk))
}

// ///////////////////////////

func TestScenario_1(t *testing.T) {
	debug.SetEnabled(true)

	// tangle := NewTestTangle(WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
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
	tf.AssertBookedCount(9, "all block should be booked")
}

func TestScenario_2(t *testing.T) {
	tf := NewTestFramework(t)
	defer tf.Shutdown()

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX1", 3, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.ledgerTf.Transaction("TX2")))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX3", 1, "TX1.0")))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX4", 1, "TX1.0")))
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block2", "Block5")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX5", 1, "TX4.0", "TX2.0")))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block1", "Block4")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX6", 1, "TX1.2")))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block7", "Block4")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX7", 1, "TX3.0", "TX6.0")))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block4", "Block7")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX8", 1, "TX1.1")))

	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block3", "Block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.checkConflictIDs(map[string]*set.AdvancedSet[utxo.TransactionID]{
		"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		"Block2": tf.ledgerTf.TransactionIDs("TX2"),
		"Block3": tf.ledgerTf.TransactionIDs("TX2"),
		"Block4": tf.ledgerTf.TransactionIDs("TX3"),
		"Block5": tf.ledgerTf.TransactionIDs("TX2", "TX4"),
		"Block6": tf.ledgerTf.TransactionIDs("TX2", "TX4"),
		"Block7": tf.ledgerTf.TransactionIDs("TX3", "TX6"),
		"Block8": tf.ledgerTf.TransactionIDs("TX3", "TX6"),
		"Block9": tf.ledgerTf.TransactionIDs("TX3", "TX6", "TX8"),
	})

	tf.checkMarkers(map[string]*markers.Markers{
		"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Block3": markers.NewMarkers(markers.NewMarker(0, 3)),
		"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block5": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Block6": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Block7": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block8": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block9": markers.NewMarkers(markers.NewMarker(0, 1)),
	})
	tf.AssertBookedCount(9, "all block should be booked")
}

func TestScenario_3(t *testing.T) {
	tf := NewTestFramework(t)
	defer tf.Shutdown()

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX1", 3, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.ledgerTf.Transaction("TX2")))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX3", 1, "TX1.0")))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithWeakParents(tf.BlockIDs("Block2")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX4", 1, "TX1.0")))
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block2", "Block5")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX5", 1, "TX4.0", "TX2.0")))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block1", "Block4")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX6", 1, "TX1.2")))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block4", "Block7")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX7", 1, "TX6.0", "TX3.0")))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block4", "Block7")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX8", 1, "TX1.1")))

	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block3", "Block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.checkConflictIDs(map[string]*set.AdvancedSet[utxo.TransactionID]{
		"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		"Block2": tf.ledgerTf.TransactionIDs("TX2"),
		"Block3": tf.ledgerTf.TransactionIDs("TX2"),
		"Block4": tf.ledgerTf.TransactionIDs("TX3"),
		"Block5": tf.ledgerTf.TransactionIDs("TX4"),
		"Block6": tf.ledgerTf.TransactionIDs("TX4", "TX2"),
		"Block7": tf.ledgerTf.TransactionIDs("TX6", "TX3"),
		"Block8": tf.ledgerTf.TransactionIDs("TX6", "TX3"),
		"Block9": tf.ledgerTf.TransactionIDs("TX6", "TX3", "TX8"),
	})
	tf.AssertBookedCount(9, "all block should be booked")
}

func TestMultiThreadedBookingAndForkingParallel(t *testing.T) {
	debug.SetEnabled(true)

	const layersNum = 127
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	// Create base-layer outputs to double-spend
	tf.CreateBlock("Block.G", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("G", layersNum, "Genesis")))
	tf.IssueBlocks("Block.G").WaitUntilAllTasksProcessed()

	blks := make([]string, 0)

	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			blkName := fmt.Sprintf("Block.%d.%d", layer, width)
			strongParents := make([]string, 0)
			if layer == 0 {
				strongParents = append(strongParents, "Block.G")
			} else {
				for innerWidth := 0; innerWidth < widthSize; innerWidth++ {
					strongParents = append(strongParents, fmt.Sprintf("Block.%d.%d", layer-1, innerWidth))
				}
			}

			var input string
			var txAlias string

			// We fork on the first two blocks for each layer
			if width < 2 {
				input = fmt.Sprintf("G.%d", layer)
				txAlias = fmt.Sprintf("TX.%d.%d", layer, width)
			}

			if input != "" {
				tf.CreateBlock(blkName, models.WithStrongParents(tf.BlockIDs(strongParents...)), models.WithPayload(tf.ledgerTf.CreateTransaction(txAlias, 1, input)))
			} else {
				tf.CreateBlock(blkName, models.WithStrongParents(tf.BlockIDs(strongParents...)))
			}

			blks = append(blks, blkName)
		}
	}

	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup

	for i := 0; i < len(blks); i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(int(50 * 1000 * rand.Float32())))
			tf.IssueBlocks(blks[i])
			wg.Done()
		}(i)
	}

	wg.Wait()
	tf.WaitUntilAllTasksProcessed()

	expectedConflicts := make(map[string]utxo.TransactionIDs)
	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			blkName := fmt.Sprintf("Block.%d.%d", layer, width)
			conflicts := make([]string, 0)

			// Add conflicts of the current layer
			if width < 2 {
				conflicts = append(conflicts, fmt.Sprintf("TX.%d.%d", layer, width))
			}

			for innerLayer := layer - 1; innerLayer >= 0; innerLayer-- {
				conflicts = append(conflicts, fmt.Sprintf("TX.%d.%d", innerLayer, 0))
				conflicts = append(conflicts, fmt.Sprintf("TX.%d.%d", innerLayer, 1))
			}

			if layer == 0 && width >= 2 {
				expectedConflicts[blkName] = utxo.NewTransactionIDs()
				continue
			}

			expectedConflicts[blkName] = tf.ledgerTf.TransactionIDs(conflicts...)
		}
	}

	tf.checkConflictIDs(expectedConflicts)
}
