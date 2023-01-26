package booker

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestScenario_1(t *testing.T) {
	tf := NewTestFramework(t)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.CreateTransaction("TX1", 3, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Transaction("TX2")))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.CreateTransaction("TX3", 1, "TX1.0")))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.CreateTransaction("TX4", 1, "TX1.0")))
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block2", "Block5")), models.WithPayload(tf.CreateTransaction("TX5", 1, "TX2.0", "TX4.0")))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block4", "Block5")), models.WithPayload(tf.Transaction("TX2")))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block4", "Block5")), models.WithPayload(tf.CreateTransaction("TX6", 1, "TX3.0", "TX4.0")))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block4", "Block6")), models.WithPayload(tf.CreateTransaction("TX7", 1, "TX5.0")))

	tf.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5", "Block6", "Block7", "Block8", "Block9").WaitUntilAllTasksProcessed()

	tf.checkConflictIDs(map[string]utxo.TransactionIDs{
		"Block1": utxo.NewTransactionIDs(),
		"Block3": utxo.NewTransactionIDs(),
		"Block2": utxo.NewTransactionIDs(),
		"Block4": tf.TransactionIDs("TX3"),
		"Block5": tf.TransactionIDs("TX4"),
		"Block6": tf.TransactionIDs("TX4", "TX5"),
		"Block7": tf.TransactionIDs("TX4", "TX3"),
		"Block8": tf.TransactionIDs("TX4", "TX3", "TX6"),
		"Block9": tf.TransactionIDs("TX4", "TX3", "TX5"),
	})
	tf.AssertBookedCount(9, "all blocks should be booked")
}

func TestScenario_2(t *testing.T) {
	tf := NewTestFramework(t)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.CreateTransaction("TX1", 3, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Transaction("TX2")))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.CreateTransaction("TX3", 1, "TX1.0")))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.CreateTransaction("TX4", 1, "TX1.0")))
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block2", "Block5")), models.WithPayload(tf.CreateTransaction("TX5", 1, "TX4.0", "TX2.0")))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block1", "Block4")), models.WithPayload(tf.CreateTransaction("TX6", 1, "TX1.2")))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block7", "Block4")), models.WithPayload(tf.CreateTransaction("TX7", 1, "TX3.0", "TX6.0")))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block4", "Block7")), models.WithPayload(tf.CreateTransaction("TX8", 1, "TX1.1")))
	tf.CreateBlock("Block0.5", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.CreateTransaction("TX9", 1, "TX8.0")))

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
		"Block0.5": tf.TransactionIDs("TX8"),
		"Block1":   utxo.NewTransactionIDs(),
		"Block2":   tf.TransactionIDs("TX2"),
		"Block3":   tf.TransactionIDs("TX2"),
		"Block4":   tf.TransactionIDs("TX3"),
		"Block5":   tf.TransactionIDs("TX2", "TX4"),
		"Block6":   tf.TransactionIDs("TX2", "TX4"),
		"Block7":   tf.TransactionIDs("TX3", "TX6"),
		"Block8":   tf.TransactionIDs("TX3", "TX6"),
		"Block9":   tf.TransactionIDs("TX3", "TX6", "TX8"),
	})

	tf.AssertBookedCount(10, "all block should be booked")
}

func TestScenario_3(t *testing.T) {
	tf := NewTestFramework(t)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.CreateTransaction("TX1", 3, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Transaction("TX2")))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.CreateTransaction("TX3", 1, "TX1.0")))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithWeakParents(tf.BlockIDs("Block2")), models.WithPayload(tf.CreateTransaction("TX4", 1, "TX1.0")))
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block2", "Block5")), models.WithPayload(tf.CreateTransaction("TX5", 1, "TX4.0", "TX2.0")))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block1", "Block4")), models.WithPayload(tf.CreateTransaction("TX6", 1, "TX1.2")))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block4", "Block7")), models.WithPayload(tf.CreateTransaction("TX7", 1, "TX6.0", "TX3.0")))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block4", "Block7")), models.WithPayload(tf.CreateTransaction("TX8", 1, "TX1.1")))

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
		"Block2": tf.TransactionIDs("TX2"),
		"Block3": tf.TransactionIDs("TX2"),
		"Block4": tf.TransactionIDs("TX3"),
		"Block5": tf.TransactionIDs("TX4"),
		"Block6": tf.TransactionIDs("TX4", "TX2"),
		"Block7": tf.TransactionIDs("TX6", "TX3"),
		"Block8": tf.TransactionIDs("TX6", "TX3"),
		"Block9": tf.TransactionIDs("TX6", "TX3", "TX8"),
	})
	tf.AssertBookedCount(9, "all block should be booked")
}

// This test step-wise tests multiple things. Where each step checks markers, block metadata diffs and conflict IDs for each block.
// 1. It tests whether a new sequence is created after max past marker gap is reached.
// 2. Propagation of conflicts through the markers, to individually mapped blocks, and across sequence boundaries.
func TestScenario_4(t *testing.T) {
	tf := NewTestFramework(t, WithBookerOptions(WithMarkerManagerOptions(markermanager.WithSequenceManagerOptions[models.BlockID, *Block](markers.WithMaxPastMarkerDistance(3)))))

	tf.CreateBlock("BaseBlock", models.WithStrongParents(tf.BlockIDs("Genesis")))
	tf.IssueBlocks("BaseBlock").WaitUntilAllTasksProcessed()

	tf.CreateBlock("Block0", models.WithStrongParents(tf.BlockIDs("BaseBlock")), models.WithPayload(tf.CreateTransaction("TX0", 4, "Genesis")))
	tf.IssueBlocks("Block0").WaitUntilAllTasksProcessed()

	markersMap := make(map[string]*markers.Markers)
	metadataDiffConflictIDs := make(map[string][]utxo.TransactionIDs)
	conflictIDs := make(map[string]utxo.TransactionIDs)

	// ISSUE Block1
	{
		tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Block0")), models.WithPayload(tf.CreateTransaction("TX1", 1, "TX0.0")))
		tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block0": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block1": markers.NewMarkers(markers.NewMarker(0, 3)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block0": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block0": utxo.NewTransactionIDs(),
			"Block1": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block2
	{
		tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")))
		tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block2": markers.NewMarkers(markers.NewMarker(0, 4)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block2": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block3
	{
		tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("BaseBlock")), models.WithPayload(tf.CreateTransaction("TX3", 1, "TX0.1")))
		tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block3": markers.NewMarkers(markers.NewMarker(0, 1)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block3": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2": utxo.NewTransactionIDs(),
			"Block3": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block4
	{
		tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithPayload(tf.CreateTransaction("TX4", 1, "TX0.0")))
		tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block4": {tf.TransactionIDs("TX4"), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block1": tf.TransactionIDs("TX1"),
			"Block2": tf.TransactionIDs("TX1"),
			"Block4": tf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block5
	{
		tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")))
		tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block5": markers.NewMarkers(markers.NewMarker(1, 2)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block5": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block5": tf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6
	{
		tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block5", "Block0")), models.WithPayload(tf.CreateTransaction("TX5", 1, "TX0.2")))
		tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6": markers.NewMarkers(markers.NewMarker(1, 3)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6": tf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6.3
	{
		tf.CreateBlock("Block6.3", models.WithStrongParents(tf.BlockIDs("Block6")))
		tf.IssueBlocks("Block6.3").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6.3": markers.NewMarkers(markers.NewMarker(1, 4)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6.3": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6.3": tf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6.6
	{
		tf.CreateBlock("Block6.6", models.WithStrongParents(tf.BlockIDs("Block6")))
		tf.IssueBlocks("Block6.6").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6.6": markers.NewMarkers(markers.NewMarker(1, 3)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6.6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6.6": tf.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block7
	{
		tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block5", "Block0")), models.WithPayload(tf.CreateTransaction("TX6", 1, "TX0.2")))
		tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7": markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(0, 2)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7": {tf.TransactionIDs("TX6"), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6":   tf.TransactionIDs("TX4", "TX5"),
			"Block6.3": tf.TransactionIDs("TX4", "TX5"),
			"Block6.6": tf.TransactionIDs("TX4", "TX5"),
			"Block7":   tf.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block7.3
	{
		tf.CreateBlock("Block7.3", models.WithStrongParents(tf.BlockIDs("Block7")))
		tf.IssueBlocks("Block7.3").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7.3": markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(0, 2)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7.3": {tf.TransactionIDs("TX6"), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block7.3": tf.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block7.6
	{
		tf.CreateBlock("Block7.6", models.WithStrongParents(tf.BlockIDs("Block7.3")))
		tf.IssueBlocks("Block7.6").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7.6": markers.NewMarkers(markers.NewMarker(2, 3)),
		}))

		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7.6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block7.6": tf.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block8
	{
		tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithPayload(tf.CreateTransaction("TX2", 1, "TX0.1")))
		tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block8": markers.NewMarkers(markers.NewMarker(0, 5)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block3": {tf.TransactionIDs("TX3"), utxo.NewTransactionIDs()},
			"Block4": {tf.TransactionIDs("TX3", "TX4"), utxo.NewTransactionIDs()},
			"Block8": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block3":   tf.TransactionIDs("TX3"),
			"Block4":   tf.TransactionIDs("TX3", "TX4"),
			"Block5":   tf.TransactionIDs("TX3", "TX4"),
			"Block6":   tf.TransactionIDs("TX3", "TX4", "TX5"),
			"Block6.3": tf.TransactionIDs("TX3", "TX4", "TX5"),
			"Block6.6": tf.TransactionIDs("TX3", "TX4", "TX5"),
			"Block7":   tf.TransactionIDs("TX3", "TX4", "TX6"),
			"Block7.3": tf.TransactionIDs("TX3", "TX4", "TX6"),
			"Block7.6": tf.TransactionIDs("TX3", "TX4", "TX6"),
			"Block8":   tf.TransactionIDs("TX2", "TX1"),
		}))
	}
}

func TestFutureConePropagation(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateBlock("Block0", models.WithStrongParents(tf.BlockIDs("Genesis")))

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Block0")), models.WithPayload(tf.CreateTransaction("TX1", 1, "Genesis")))
	tf.CreateBlock("Block1*", models.WithStrongParents(tf.BlockIDs("Block0")), models.WithPayload(tf.CreateTransaction("TX1*", 1, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithPayload(tf.CreateTransaction("TX2", 1, "TX1.0")))
	tf.CreateBlock("Block2*", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithPayload(tf.CreateTransaction("TX2*", 1, "TX1.0")))

	// these are all liking TX1* -> should not have TX1 and TX2
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithLikedInsteadParents(tf.BlockIDs("Block1*"))) // propagation via markers
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithLikedInsteadParents(tf.BlockIDs("Block1*"))) // check correct booking (later)
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithLikedInsteadParents(tf.BlockIDs("Block1*"))) // propagation via individually mapped blocks
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block5")))                                                         // propagation via individually mapped blocks

	// these are liking TX1 -> should have TX2 too
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block2")))                                                        // propagation via individually mapped blocks
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithLikedInsteadParents(tf.BlockIDs("Block1"))) // propagation via marker
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithLikedInsteadParents(tf.BlockIDs("Block1"))) // propagation via marker

	markersMap := make(map[string]*markers.Markers)
	metadataDiffConflictIDs := make(map[string][]utxo.TransactionIDs)
	conflictIDs := make(map[string]utxo.TransactionIDs)

	{
		tf.IssueBlocks("Block0").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block1*").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block5", "Block6").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block7", "Block8", "Block9").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block0":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block1":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block1*": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block3":  markers.NewMarkers(markers.NewMarker(0, 4)),
			"Block5":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block6":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block7":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block8":  markers.NewMarkers(markers.NewMarker(0, 5)),
			"Block9":  markers.NewMarkers(markers.NewMarker(0, 3)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block1":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1*": {tf.TransactionIDs("TX1*"), utxo.NewTransactionIDs()},
			"Block2":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block3":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block5":  {tf.TransactionIDs("TX1*"), tf.TransactionIDs("TX1")},
			"Block6":  {tf.TransactionIDs("TX1*"), tf.TransactionIDs("TX1")},
			"Block7":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block8":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block9":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block1":  tf.TransactionIDs("TX1"),
			"Block1*": tf.TransactionIDs("TX1*"),
			"Block2":  tf.TransactionIDs("TX1"),
			"Block3":  tf.TransactionIDs("TX1*"),
			"Block5":  tf.TransactionIDs("TX1*"),
			"Block6":  tf.TransactionIDs("TX1*"),
			"Block7":  tf.TransactionIDs("TX1"),
			"Block8":  tf.TransactionIDs("TX1"),
			"Block9":  tf.TransactionIDs("TX1"),
		}))
	}

	// Verify correct propagation of TX2 to its future cone.
	{
		tf.IssueBlocks("Block2*").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block2*": markers.NewMarkers(markers.NewMarker(0, 2)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block2*": {tf.TransactionIDs("TX2*"), utxo.NewTransactionIDs()},
			"Block5":  {tf.TransactionIDs("TX1*"), tf.TransactionIDs("TX1")},
			"Block6":  {tf.TransactionIDs("TX1*"), tf.TransactionIDs("TX1")},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2":  tf.TransactionIDs("TX1", "TX2"),
			"Block2*": tf.TransactionIDs("TX1", "TX2*"),
			"Block3":  tf.TransactionIDs("TX1*"), // does not change because of marker mapping
			"Block5":  tf.TransactionIDs("TX1*"), // does not change because of additional diff entry
			"Block6":  tf.TransactionIDs("TX1*"), // does not change because of additional diff entry
			"Block7":  tf.TransactionIDs("TX1", "TX2"),
			"Block8":  tf.TransactionIDs("TX1", "TX2"),
			"Block9":  tf.TransactionIDs("TX1", "TX2"),
		}))
	}

	// Check that inheritance works as expected when booking Block4.
	{
		tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block4": markers.NewMarkers(markers.NewMarker(0, 3)),
		}))
		tf.checkBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block4": {tf.TransactionIDs("TX1*"), tf.TransactionIDs("TX1", "TX2")},
		}))
		tf.checkConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block4": tf.TransactionIDs("TX1*"),
		}))
	}
}

func TestWeakParent(t *testing.T) {
	tf := NewTestFramework(t)
	tf.CreateBlock("Block0", models.WithStrongParents(tf.BlockIDs("Genesis")))

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Block0")), models.WithPayload(tf.CreateTransaction("TX1", 1, "Genesis")))
	tf.CreateBlock("Block1*", models.WithStrongParents(tf.BlockIDs("Block0")), models.WithPayload(tf.CreateTransaction("TX1*", 1, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block0")), models.WithWeakParents(tf.BlockIDs("Block1")))

	{
		tf.IssueBlocks("Block0").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block1*").WaitUntilAllTasksProcessed()
		tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		tf.checkBlockMetadataDiffConflictIDs(map[string][]utxo.TransactionIDs{
			"Block0":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1*": {tf.TransactionIDs("TX1*"), utxo.NewTransactionIDs()},
			"Block2":  {tf.TransactionIDs("TX1"), utxo.NewTransactionIDs()},
		})
		tf.checkConflictIDs(map[string]utxo.TransactionIDs{
			"Block0":  tf.TransactionIDs(),
			"Block1":  tf.TransactionIDs("TX1"),
			"Block1*": tf.TransactionIDs("TX1*"),
			"Block2":  tf.TransactionIDs("TX1"),
		})
	}
}

func TestMultiThreadedBookingAndForkingParallel(t *testing.T) {
	const layersNum = 127
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	tf := NewTestFramework(t)

	// Create base-layer outputs to double-spend
	tf.CreateBlock("Block.G", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.CreateTransaction("G", layersNum, "Genesis")))
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
				tf.CreateBlock(blkName, models.WithStrongParents(tf.BlockIDs(strongParents...)), models.WithPayload(tf.CreateTransaction(txAlias, 1, input)))
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
	event.Loop.PendingTasksCounter.WaitIsZero()

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

			expectedConflicts[blkName] = tf.TransactionIDs(conflicts...)
		}
	}

	tf.checkConflictIDs(expectedConflicts)
}

func TestMultiThreadedBookingAndForkingNested(t *testing.T) {
	const layersNum = 50
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	tf := NewTestFramework(t)

	// Create base-layer outputs to double-spend
	tf.CreateBlock("Block.G", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithPayload(tf.CreateTransaction("G", widthSize, "Genesis")))
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
			tf.CreateBlock(blkName, models.WithStrongParents(tf.BlockIDs(strongParents...)), models.WithLikedInsteadParents(tf.BlockIDs(likeParents...)), models.WithPayload(tf.CreateTransaction(txAlias, 1, input)))

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
	event.Loop.PendingTasksCounter.WaitIsZero()

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

			expectedConflicts[blkName] = tf.TransactionIDs(conflicts...)
		}
	}

	tf.checkNormalizedConflictIDsContained(expectedConflicts)
}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When pruning the BlockDAG, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func Test_Prune(t *testing.T) {
	const epochCount = 100

	tf := NewTestFramework(t)

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 1 {
			fmt.Println("Creating genesis block")

			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
				models.WithPayload(tf.CreateTransaction(alias, 1, "Genesis")),
			), alias
		}
		parentAlias := fmt.Sprintf("blk%s-%d", prefix, idx-1)
		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(parentAlias)),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx-1)*epoch.Duration, 0)),
			models.WithPayload(tf.CreateTransaction(alias, 1, fmt.Sprintf("%s.0", parentAlias))),
		), alias
	}

	require.EqualValues(t, 0, tf.BlockDAG.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch should be 0")

	expectedInvalid := make(map[string]bool, epochCount)
	expectedBooked := make(map[string]bool, epochCount)

	// Attach solid blocks
	for i := 1; i <= epochCount; i++ {
		block, alias := createNewBlock(i, "")

		_, wasAttached, err := tf.BlockDAG.Attach(block)
		require.True(t, wasAttached, "block should be attached")
		require.NoError(t, err, "should not be able to attach a block after shutdown")

		if i >= epochCount/4 {
			expectedInvalid[alias] = false
			expectedBooked[alias] = true
		}
	}

	_, wasAttached, err := tf.BlockDAG.Attach(tf.CreateBlock(
		"blk-1-reattachment",
		models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk-%d", epochCount))),
		models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(epochCount)*epoch.Duration, 0)),
		models.WithPayload(tf.Transaction("blk-1")),
	))
	require.True(t, wasAttached, "block should be attached")
	require.NoError(t, err, "should not be able to attach a block after shutdown")

	event.Loop.PendingTasksCounter.WaitIsZero()

	tf.AssertBookedCount(epochCount+1, "should have all solid blocks")

	validateState(tf, 0, epochCount)

	tf.BlockDAG.EvictionState.EvictUntil(epochCount / 4)
	event.Loop.PendingTasksCounter.WaitIsZero()

	require.EqualValues(t, epochCount/4, tf.BlockDAG.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch of booker should be epochCount/4")

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	tf.AssertInvalidCount(0, "should have invalid blocks")

	tf.BlockDAG.EvictionState.EvictUntil(epochCount / 10)
	event.Loop.PendingTasksCounter.WaitIsZero()

	require.EqualValues(t, epochCount/4, tf.BlockDAG.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch of booker should be epochCount/4")

	tf.BlockDAG.EvictionState.EvictUntil(epochCount / 2)
	event.Loop.PendingTasksCounter.WaitIsZero()

	require.EqualValues(t, epochCount/2, tf.BlockDAG.EvictionState.LastEvictedEpoch(), "maxDroppedEpoch of booker should be epochCount/2")

	validateState(tf, epochCount/2, epochCount)

	_, wasAttached, err = tf.BlockDAG.Attach(tf.CreateBlock(
		"blk-0.5",
		models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk-%d", epochCount))),
		models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
	))
	event.Loop.PendingTasksCounter.WaitIsZero()

	require.False(t, wasAttached, "block should not be attached")
	require.Error(t, err, "should not be able to attach a block after eviction of an epoch")
}

func validateState(tf *TestFramework, maxPrunedEpoch, epochCount int) {
	for i := maxPrunedEpoch + 1; i <= epochCount; i++ {
		alias := fmt.Sprintf("blk-%d", i)

		_, exists := tf.Booker.Block(tf.ModelsTestFramework.Block(alias).ID())
		require.True(tf.test, exists, "block should be in the BlockDAG")
		if i == 1 {
			blocks := tf.Booker.attachments.Get(tf.Transaction(alias).ID())
			require.Len(tf.test, blocks, 2, "transaction blk-0 should have 2 attachments")
		} else {
			blocks := tf.Booker.attachments.Get(tf.Transaction(alias).ID())
			require.Len(tf.test, blocks, 1, "transaction should have 1 attachment")
		}
	}

	for i := 1; i <= maxPrunedEpoch; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		_, exists := tf.Booker.Block(tf.ModelsTestFramework.Block(alias).ID())
		require.False(tf.test, exists, "block should not be in the BlockDAG")
		if i == 1 {
			blocks := tf.Booker.attachments.Get(tf.Transaction(alias).ID())
			require.Len(tf.test, blocks, 1, "transaction should have 1 attachment")
		} else {
			blocks := tf.Booker.attachments.Get(tf.Transaction(alias).ID())
			require.Empty(tf.test, blocks, "transaction should have no attachments")
		}
	}
}

func Test_BlockInvalid(t *testing.T) {
	tf := NewTestFramework(t)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithLikedInsteadParents(tf.BlockIDs("Block1")))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Genesis", "Block1")))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block1", "Block2")))
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block2", "Block5")))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block4", "Block5")))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block4", "Block5")))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block4", "Block6")))

	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.AssertInvalidCount(7, "8 blocks should be invalid")
	tf.AssertBookedCount(2, "1 block should be booked")
	tf.AssertInvalid(map[string]bool{
		"Block1": false,
		"Block2": true,
		"Block3": true,
		"Block4": false,
		"Block5": true,
		"Block6": true,
		"Block7": true,
		"Block8": true,
		"Block9": true,
	})

	tf.AssertBooked(map[string]bool{
		"Block1": true,
		"Block2": false, // TODO: set as booked when actually booked instead of setting as booked by causal order
		"Block3": false,
		"Block4": true,
		"Block5": false,
		"Block6": false,
		"Block7": false,
		"Block8": false,
	})

	tf.AssertSolid(map[string]bool{
		"Block1": true,
		"Block2": true,
		"Block3": true,
		"Block4": true,
		"Block5": true,
		"Block6": true,
		"Block7": true,
		"Block8": true,
		"Block9": false, // Block9 is not solid because it is invalid
	})
}
