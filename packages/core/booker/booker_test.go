package booker

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
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
	tf.CreateBlock("Block0.5", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("TX9", 1, "TX8.0")))

	tf.IssueBlocks("Block0.5").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block3", "Block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.checkConflictIDs(map[string]utxo.TransactionIDs{
		"Block0.5": tf.ledgerTf.TransactionIDs("TX8"),
		"Block1":   utxo.NewTransactionIDs(),
		"Block2":   tf.ledgerTf.TransactionIDs("TX2"),
		"Block3":   tf.ledgerTf.TransactionIDs("TX2"),
		"Block4":   tf.ledgerTf.TransactionIDs("TX3"),
		"Block5":   tf.ledgerTf.TransactionIDs("TX2", "TX4"),
		"Block6":   tf.ledgerTf.TransactionIDs("TX2", "TX4"),
		"Block7":   tf.ledgerTf.TransactionIDs("TX3", "TX6"),
		"Block8":   tf.ledgerTf.TransactionIDs("TX3", "TX6"),
		"Block9":   tf.ledgerTf.TransactionIDs("TX3", "TX6", "TX8"),
	})

	tf.AssertBookedCount(10, "all block should be booked")
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

func TestMultiThreadedBookingAndForkingNested(t *testing.T) {
	const layersNum = 50
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	// Create base-layer outputs to double-spend
	tf.CreateBlock("Block.G", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.ledgerTf.CreateTransaction("G", widthSize, "Genesis")))
	tf.IssueBlocks("Block.G").WaitUntilAllTasksProcessed()

	blks := make([]string, 0)

	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			blkName := fmt.Sprintf("Block.%d.%d", layer, width)
			strongParents := make([]string, 0)
			likeParents := make([]string, 0)
			if layer == 0 {
				strongParents = append(strongParents, "Block.G")
			} else {
				for innerWidth := 0; innerWidth < widthSize; innerWidth++ {
					strongParents = append(strongParents, fmt.Sprintf("Block.%d.%d", layer-1, innerWidth))
					// We only like the first conflict over the second, to fork it on the next layer.
					if innerWidth%2 == 0 {
						likeParents = append(likeParents, fmt.Sprintf("Block.%d.%d", layer-1, innerWidth))
					}
				}
			}

			var input string
			if layer == 0 {
				input = fmt.Sprintf("G.%d", width-width%2)
			} else {
				// We spend from the first of the couple forks
				input = fmt.Sprintf("TX.%d.%d.0", layer-1, width-width%2)
			}
			txAlias := fmt.Sprintf("TX.%d.%d", layer, width)
			tf.CreateBlock(blkName, models.WithStrongParents(tf.BlockIDs(strongParents...)), models.WithLikedInsteadParents(tf.BlockIDs(likeParents...)), models.WithPayload(tf.ledgerTf.CreateTransaction(txAlias, 1, input)))

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

			// Add conflict of the current layer
			conflicts = append(conflicts, fmt.Sprintf("TX.%d.%d", layer, width))

			// Add conflicts of the previous layers
			for innerLayer := layer - 1; innerLayer >= 0; innerLayer-- {
				for innerWidth := 0; innerWidth < widthSize; innerWidth += 2 {
					conflicts = append(conflicts, fmt.Sprintf("TX.%d.%d", innerLayer, innerWidth))
				}
			}

			expectedConflicts[blkName] = tf.ledgerTf.TransactionIDs(conflicts...)
		}
	}

	checkNormalizedConflictIDsContained(t, tf, expectedConflicts)
}

func checkNormalizedConflictIDsContained(t *testing.T, tf *TestFramework, expectedContainedConflictIDs map[string]utxo.TransactionIDs) {
	for blockID, blockExpectedConflictIDs := range expectedContainedConflictIDs {
		_, blockConflictIDs := tf.Booker.blockBookingDetails(tf.Block(blockID))

		normalizedRetrievedConflictIDs := blockConflictIDs.Clone()
		for it := blockConflictIDs.Iterator(); it.HasNext(); {
			tf.Booker.ledger.ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				normalizedRetrievedConflictIDs.DeleteAll(b.Parents())
			})
		}

		normalizedExpectedConflictIDs := blockExpectedConflictIDs.Clone()
		for it := blockExpectedConflictIDs.Iterator(); it.HasNext(); {
			tf.Booker.ledger.ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				normalizedExpectedConflictIDs.DeleteAll(b.Parents())
			})
		}

		assert.True(t, normalizedExpectedConflictIDs.Intersect(normalizedRetrievedConflictIDs).Size() == normalizedExpectedConflictIDs.Size(), "ConflictID of %s should be %s but is %s", blockID, normalizedExpectedConflictIDs, normalizedRetrievedConflictIDs)
	}
}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When pruning the tangle, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func Test_Prune(t *testing.T) {
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 0 {
			fmt.Println("Creating genesis block")

			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
				models.WithPayload(tf.ledgerTf.CreateTransaction(alias, 1, "Genesis")),
			), alias
		}
		parentAlias := fmt.Sprintf("blk%s-%d", prefix, idx-1)
		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(parentAlias)),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)),
			models.WithPayload(tf.ledgerTf.CreateTransaction(alias, 1, fmt.Sprintf("%s.0", parentAlias))),
		), alias
	}

	assert.EqualValues(t, -1, tf.Booker.maxDroppedEpoch, "maxDroppedEpoch should be 0")

	expectedInvalid := make(map[string]bool, epochCount)
	expectedBooked := make(map[string]bool, epochCount)

	// Attach solid blocks
	for i := 0; i < epochCount; i++ {
		block, alias := createNewBlock(i, "")

		_, wasAttached, err := tf.Tangle.Attach(block)
		assert.True(t, wasAttached, "block should be attached")
		assert.NoError(t, err, "should not be able to attach a block after shutdown")

		if i > epochCount/4 {
			expectedInvalid[alias] = false
			expectedBooked[alias] = true
		}
	}

	_, wasAttached, err := tf.Tangle.Attach(tf.CreateBlock(
		"blk-0-reattachment",
		models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk-%d", epochCount-1))),
		models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(epochCount-1)*epoch.Duration, 0)),
		models.WithPayload(tf.ledgerTf.Transaction("blk-0")),
	))
	assert.True(t, wasAttached, "block should be attached")
	assert.NoError(t, err, "should not be able to attach a block after shutdown")

	tf.WaitUntilAllTasksProcessed()

	tf.AssertBookedCount(epochCount+1, "should have all solid blocks")

	validateState(tf, -1, epochCount)

	tf.Booker.Prune(epochCount / 4)
	tf.WaitUntilAllTasksProcessed()

	assert.EqualValues(t, epochCount/4, tf.Booker.maxDroppedEpoch, "maxDroppedEpoch of booker should be epochCount/4")
	assert.EqualValues(t, epochCount/4, tf.Booker.markerManager.maxDroppedEpoch, "maxDroppedEpoch of markersManager should be %d", epochCount/4)
	assert.EqualValues(t, epochCount/4, tf.Booker.attachments.maxDroppedEpoch, "maxDroppedEpoch of attachments should be %d", epochCount/4)

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	tf.AssertInvalidCount(0, "should have invalid blocks")

	tf.Booker.Prune(epochCount / 10)

	assert.EqualValues(t, epochCount/4, tf.Booker.maxDroppedEpoch, "maxDroppedEpoch of booker should be epochCount/4")
	assert.EqualValues(t, epochCount/4, tf.Booker.markerManager.maxDroppedEpoch, "maxDroppedEpoch of markersManager should be %d", epochCount/4)
	assert.EqualValues(t, epochCount/4, tf.Booker.attachments.maxDroppedEpoch, "maxDroppedEpoch of attachments should be %d", epochCount/4)

	tf.Booker.Prune(epochCount / 2)
	assert.EqualValues(t, epochCount/2, tf.Booker.maxDroppedEpoch, "maxDroppedEpoch of booker should be epochCount/2")
	assert.EqualValues(t, epochCount/2, tf.Booker.markerManager.maxDroppedEpoch, "maxDroppedEpoch of markersManager should be %d", epochCount/2)
	assert.EqualValues(t, epochCount/2, tf.Booker.attachments.maxDroppedEpoch, "maxDroppedEpoch of attachments should be %d", epochCount/2)

	validateState(tf, epochCount/2, epochCount)

	_, wasAttached, err = tf.Tangle.Attach(tf.CreateBlock(
		"blk-0.5",
		models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk-%d", epochCount-1))),
		models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
	))
	tf.WaitUntilAllTasksProcessed()

	assert.True(t, wasAttached, "block should be attached")
	assert.NoError(t, err, "should not be able to attach a block after shutdown")

	_, exists := tf.Booker.Block(tf.TestFramework.Block("blk-0.5").ID())
	assert.False(t, exists, "blk-0.5 should not be booked")
}

func validateState(tf *TestFramework, maxPrunedEpoch, epochCount int) {
	for i := maxPrunedEpoch + 1; i < epochCount; i++ {
		alias := fmt.Sprintf("blk-%d", i)

		_, exists := tf.Booker.Block(tf.TestFramework.Block(alias).ID())
		assert.True(tf.T, exists, "block should be in the tangle")
		if i == 0 {
			blocks := tf.Booker.attachments.Get(tf.ledgerTf.Transaction(alias).ID())
			assert.Len(tf.T, blocks, 2, "transaction blk-0 should have 2 attachments")
		} else {
			blocks := tf.Booker.attachments.Get(tf.ledgerTf.Transaction(alias).ID())
			assert.Len(tf.T, blocks, 1, "transaction should have 1 attachment")
		}
	}

	for i := 0; i < maxPrunedEpoch; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		_, exists := tf.Booker.Block(tf.TestFramework.Block(alias).ID())
		assert.False(tf.T, exists, "block should not be in the tangle")
		if i == 0 {
			blocks := tf.Booker.attachments.Get(tf.ledgerTf.Transaction(alias).ID())
			assert.Len(tf.T, blocks, 1, "transaction should have 1 attachment")
		} else {
			blocks := tf.Booker.attachments.Get(tf.ledgerTf.Transaction(alias).ID())
			assert.Empty(tf.T, blocks, "transaction should have no attachments")
		}
	}
}
