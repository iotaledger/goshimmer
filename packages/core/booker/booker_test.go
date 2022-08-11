package booker

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

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

	tf.checkConflictIDs(map[string]utxo.TransactionIDs{
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

	tf.checkConflictIDs(map[string]utxo.TransactionIDs{
		"Block1": utxo.NewTransactionIDs(),
		"Block2": tf.ledgerTf.TransactionIDs("TX2"),
		"Block3": tf.ledgerTf.TransactionIDs("TX2"),
		"Block4": tf.ledgerTf.TransactionIDs("TX3"),
		"Block5": tf.ledgerTf.TransactionIDs("TX2", "TX4"),
		"Block6": tf.ledgerTf.TransactionIDs("TX2", "TX4"),
		"Block7": tf.ledgerTf.TransactionIDs("TX3", "TX6"),
		"Block8": tf.ledgerTf.TransactionIDs("TX3", "TX6"),
		"Block9": tf.ledgerTf.TransactionIDs("TX3", "TX6", "TX8"),
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

	tf.checkConflictIDs(map[string]utxo.TransactionIDs{
		"Block1": utxo.NewTransactionIDs(),
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

// This test step-wise tests multiple things. Where each step checks markers, block metadata diffs and conflict IDs for each block.
// 1. It tests whether a new sequence is created after max past marker gap is reached.
// 2. Propagation of conflicts through the markers, to individually mapped blocks, and across sequence boundaries.
func TestScenario_4(t *testing.T) {
	tf := NewTestFramework(t)
	defer tf.Shutdown()
	tf.CreateBlock("Block0", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX0", 4, "Genesis")))
	tf.IssueBlocks("Block0").WaitUntilAllTasksProcessed()

	markersMap := make(map[string]*markers.Markers)
	metadataDiffConflictIDs := make(map[string][]utxo.TransactionIDs)
	conflictIDs := make(map[string]utxo.TransactionIDs)

	// ISSUE Block1
	{
		tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Block0")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX1", 1, "TX0.0")))
		tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block0": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block1": markers.NewMarkers(markers.NewMarker(0, 2)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block0": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block0": utxo.NewTransactionIDs(),
			"Block1": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block2
	{
		tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")))
		tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block2": markers.NewMarkers(markers.NewMarker(0, 3)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block2": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block3
	{
		tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX3", 1, "TX0.1")))
		tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block3": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2": utxo.NewTransactionIDs(),
			"Block3": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block4
	{
		tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX4", 1, "TX0.0")))
		tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block4": {tf.ledgerTf.TransactionIDs("TX4"), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block1": tf.ledgerTf.TransactionIDs("TX1"),
			"Block2": tf.ledgerTf.TransactionIDs("TX1"),
			"Block4": tf.ledgerTf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block5
	{
		tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")))
		tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block5": markers.NewMarkers(markers.NewMarker(1, 1)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block5": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block5": tf.ledgerTf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6
	{
		tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block5", "Block0")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX5", 1, "TX0.2")))
		tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6": markers.NewMarkers(markers.NewMarker(1, 2)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6": tf.ledgerTf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6.3
	{
		tf.CreateBlock("Block6.3", models.WithStrongParents(tf.BlockIDs("Block6")))
		tf.IssueBlocks("Block6.3").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6.3": markers.NewMarkers(markers.NewMarker(1, 3)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6.3": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6.3": tf.ledgerTf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6.6
	{
		tf.CreateBlock("Block6.6", models.WithStrongParents(tf.BlockIDs("Block6")))
		tf.IssueBlocks("Block6.6").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6.6": markers.NewMarkers(markers.NewMarker(1, 2)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6.6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6.6": tf.ledgerTf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block7
	{
		tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block5", "Block0")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX6", 1, "TX0.2")))
		tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7": markers.NewMarkers(markers.NewMarker(1, 1), markers.NewMarker(0, 1)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7": {tf.ledgerTf.TransactionIDs("TX6"), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6":   tf.ledgerTf.TransactionIDs("TX4", "TX5"),
			"Block6.3": tf.ledgerTf.TransactionIDs("TX4", "TX5"),
			"Block6.6": tf.ledgerTf.TransactionIDs("TX4", "TX5"),
			"Block7":   tf.ledgerTf.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block7.3
	{
		tf.CreateBlock("Block7.3", models.WithStrongParents(tf.BlockIDs("Block7")))
		tf.IssueBlocks("Block7.3").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7.3": markers.NewMarkers(markers.NewMarker(1, 1), markers.NewMarker(0, 1)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7.3": {tf.ledgerTf.TransactionIDs("TX6"), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block7.3": tf.ledgerTf.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block7.6
	{
		tf.CreateBlock("Block7.6", models.WithStrongParents(tf.BlockIDs("Block7.3")))
		tf.IssueBlocks("Block7.6").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7.6": markers.NewMarkers(markers.NewMarker(2, 2)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7.6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block7.6": tf.ledgerTf.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block8
	{
		tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX2", 1, "TX0.1")))
		tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

		tf.checkMarkers(MergeMaps(markersMap, map[string]*markers.Markers{
			"Block8": markers.NewMarkers(markers.NewMarker(0, 4)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block3": {tf.ledgerTf.TransactionIDs("TX3"), utxo.NewTransactionIDs()},
			"Block4": {tf.ledgerTf.TransactionIDs("TX3", "TX4"), utxo.NewTransactionIDs()},
			"Block8": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block3":   tf.ledgerTf.TransactionIDs("TX3"),
			"Block4":   tf.ledgerTf.TransactionIDs("TX3", "TX4"),
			"Block5":   tf.ledgerTf.TransactionIDs("TX3", "TX4"),
			"Block6":   tf.ledgerTf.TransactionIDs("TX3", "TX4", "TX5"),
			"Block6.3": tf.ledgerTf.TransactionIDs("TX3", "TX4", "TX5"),
			"Block6.6": tf.ledgerTf.TransactionIDs("TX3", "TX4", "TX5"),
			"Block7":   tf.ledgerTf.TransactionIDs("TX3", "TX4", "TX6"),
			"Block7.3": tf.ledgerTf.TransactionIDs("TX3", "TX4", "TX6"),
			"Block7.6": tf.ledgerTf.TransactionIDs("TX3", "TX4", "TX6"),
			"Block8":   tf.ledgerTf.TransactionIDs("TX2", "TX1"),
		}))
	}
}

// TODO:
//  1. implement test with invalidity
//  2. implement test with future cone dislike
//  3. implement the other multithreaded test

func MergeMaps[K comparable, V any](base, update map[K]V) map[K]V {
	for k, v := range update {
		base[k] = v
	}
	return base
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
