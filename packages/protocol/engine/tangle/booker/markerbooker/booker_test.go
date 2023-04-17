package markerbooker_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/realitiesledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag/inmemoryblockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func TestScenario_1(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t,
		workers.CreateGroup("BookerTestFramework"),
		realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("TX1", 3, "Genesis")))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Ledger.Transaction("TX2")))
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX3", 1, "TX1.0")))
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Ledger.CreateTransaction("TX4", 1, "TX1.0")))
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2", "Block5")), models.WithPayload(tf.Ledger.CreateTransaction("TX5", 1, "TX2.0", "TX4.0")))
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block5")), models.WithPayload(tf.Ledger.Transaction("TX2")))
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block5")), models.WithPayload(tf.Ledger.CreateTransaction("TX6", 1, "TX3.0", "TX4.0")))
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block6")), models.WithPayload(tf.Ledger.CreateTransaction("TX7", 1, "TX5.0")))

	tf.BlockDAG.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5", "Block6", "Block7", "Block8", "Block9")

	workers.WaitChildren()

	tf.CheckConflictIDs(map[string]utxo.TransactionIDs{
		"Block1": utxo.NewTransactionIDs(),
		"Block3": utxo.NewTransactionIDs(),
		"Block2": utxo.NewTransactionIDs(),
		"Block4": tf.Ledger.TransactionIDs("TX3"),
		"Block5": tf.Ledger.TransactionIDs("TX4"),
		"Block6": tf.Ledger.TransactionIDs("TX4", "TX5"),
		"Block7": tf.Ledger.TransactionIDs("TX4", "TX3"),
		"Block8": tf.Ledger.TransactionIDs("TX4", "TX3", "TX6"),
		"Block9": tf.Ledger.TransactionIDs("TX4", "TX3", "TX5"),
	})
	tf.AssertBookedCount(9, "all blocks should be booked")
}

func TestScenario_2(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("TX1", 3, "Genesis")))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Ledger.Transaction("TX2")))
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX3", 1, "TX1.0")))
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Ledger.CreateTransaction("TX4", 1, "TX1.0")))
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2", "Block5")), models.WithPayload(tf.Ledger.CreateTransaction("TX5", 1, "TX4.0", "TX2.0")))
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block4")), models.WithPayload(tf.Ledger.CreateTransaction("TX6", 1, "TX1.2")))
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7", "Block4")), models.WithPayload(tf.Ledger.CreateTransaction("TX7", 1, "TX3.0", "TX6.0")))
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block7")), models.WithPayload(tf.Ledger.CreateTransaction("TX8", 1, "TX1.1")))
	tf.BlockDAG.CreateBlock("Block0.5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("TX9", 1, "TX8.0")))

	tf.BlockDAG.IssueBlocks("Block0.5")
	tf.BlockDAG.IssueBlocks("Block1")
	tf.BlockDAG.IssueBlocks("Block2")
	tf.BlockDAG.IssueBlocks("Block3", "Block4")
	tf.BlockDAG.IssueBlocks("Block5")
	tf.BlockDAG.IssueBlocks("Block6")
	tf.BlockDAG.IssueBlocks("Block7")
	tf.BlockDAG.IssueBlocks("Block8")
	tf.BlockDAG.IssueBlocks("Block9")

	workers.WaitChildren()

	tf.CheckConflictIDs(map[string]utxo.TransactionIDs{
		"Block0.5": tf.Ledger.TransactionIDs("TX8"),
		"Block1":   utxo.NewTransactionIDs(),
		"Block2":   tf.Ledger.TransactionIDs("TX2"),
		"Block3":   tf.Ledger.TransactionIDs("TX2"),
		"Block4":   tf.Ledger.TransactionIDs("TX3"),
		"Block5":   tf.Ledger.TransactionIDs("TX2", "TX4"),
		"Block6":   tf.Ledger.TransactionIDs("TX2", "TX4"),
		"Block7":   tf.Ledger.TransactionIDs("TX3", "TX6"),
		"Block8":   tf.Ledger.TransactionIDs("TX3", "TX6"),
		"Block9":   tf.Ledger.TransactionIDs("TX3", "TX6", "TX8"),
	})

	tf.AssertBookedCount(10, "all block should be booked")
}

func TestScenario_3(t *testing.T) {
	t.Skip("Skip until we propagate conflicts through Markers again")
	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("TX1", 3, "Genesis")))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX2", 1, "TX1.1", "TX1.2")))
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block2")), models.WithPayload(tf.Ledger.Transaction("TX2")))
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis", "Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX3", 1, "TX1.0")))
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithWeakParents(tf.BlockDAG.BlockIDs("Block2")), models.WithPayload(tf.Ledger.CreateTransaction("TX4", 1, "TX1.0")))
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2", "Block5")), models.WithPayload(tf.Ledger.CreateTransaction("TX5", 1, "TX4.0", "TX2.0")))
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block4")), models.WithPayload(tf.Ledger.CreateTransaction("TX6", 1, "TX1.2")))
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block7")), models.WithPayload(tf.Ledger.CreateTransaction("TX7", 1, "TX6.0", "TX3.0")))
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block7")), models.WithPayload(tf.Ledger.CreateTransaction("TX8", 1, "TX1.1")))

	tf.BlockDAG.IssueBlocks("Block1")
	tf.BlockDAG.IssueBlocks("Block2")
	tf.BlockDAG.IssueBlocks("Block3", "Block4")
	tf.BlockDAG.IssueBlocks("Block5")
	tf.BlockDAG.IssueBlocks("Block6")
	tf.BlockDAG.IssueBlocks("Block7")
	tf.BlockDAG.IssueBlocks("Block8")
	tf.BlockDAG.IssueBlocks("Block9")

	tf.CheckConflictIDs(map[string]utxo.TransactionIDs{
		"Block1": utxo.NewTransactionIDs(),
		"Block2": tf.Ledger.TransactionIDs("TX2"),
		"Block3": tf.Ledger.TransactionIDs("TX2"),
		"Block4": tf.Ledger.TransactionIDs("TX3"),
		"Block5": tf.Ledger.TransactionIDs("TX4"),
		"Block6": tf.Ledger.TransactionIDs("TX4", "TX2"),
		"Block7": tf.Ledger.TransactionIDs("TX6", "TX3"),
		"Block8": tf.Ledger.TransactionIDs("TX6", "TX3"),
		"Block9": tf.Ledger.TransactionIDs("TX6", "TX3", "TX8"),
	})
	tf.AssertBookedCount(9, "all block should be booked")
}

// This test step-wise tests multiple things. Where each step checks markers, block metadata diffs and conflict IDs for each block.
// 1. It tests whether a new sequence is created after max past marker gap is reached.
// 2. Propagation of conflicts through the markers, to individually mapped blocks, and across sequence boundaries.
func TestScenario_4(t *testing.T) {
	t.Skip("Skip until we propagate conflicts through Markers again")
	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"),
		realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")),
		markerbooker.WithMarkerManagerOptions(
			markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3)),
		),
	)

	tf.BlockDAG.CreateBlock("BaseBlock", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")))
	tf.BlockDAG.IssueBlocks("BaseBlock")

	tf.BlockDAG.CreateBlock("Block0", models.WithStrongParents(tf.BlockDAG.BlockIDs("BaseBlock")), models.WithPayload(tf.Ledger.CreateTransaction("TX0", 4, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block0")

	markersMap := make(map[string]*markers.Markers)
	metadataDiffConflictIDs := make(map[string][]utxo.TransactionIDs)
	conflictIDs := make(map[string]utxo.TransactionIDs)

	// ISSUE Block1
	{
		tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block0")), models.WithPayload(tf.Ledger.CreateTransaction("TX1", 1, "TX0.0")))
		tf.BlockDAG.IssueBlocks("Block1")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block0": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block1": markers.NewMarkers(markers.NewMarker(0, 3)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block0": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block0": utxo.NewTransactionIDs(),
			"Block1": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block2
	{
		tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")))
		tf.BlockDAG.IssueBlocks("Block2")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block2": markers.NewMarkers(markers.NewMarker(0, 4)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block2": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block3
	{
		tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("BaseBlock")), models.WithPayload(tf.Ledger.CreateTransaction("TX3", 1, "TX0.1")))
		tf.BlockDAG.IssueBlocks("Block3")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block3": markers.NewMarkers(markers.NewMarker(0, 1)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block3": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2": utxo.NewTransactionIDs(),
			"Block3": utxo.NewTransactionIDs(),
		}))
	}

	// ISSUE Block4
	{
		tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithPayload(tf.Ledger.CreateTransaction("TX4", 1, "TX0.0")))
		tf.BlockDAG.IssueBlocks("Block4")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block4": {tf.Ledger.TransactionIDs("TX4"), utxo.NewTransactionIDs()},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block1": tf.Ledger.TransactionIDs("TX1"),
			"Block2": tf.Ledger.TransactionIDs("TX1"),
			"Block4": tf.Ledger.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block5
	{
		tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")))
		tf.BlockDAG.IssueBlocks("Block5")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block5": markers.NewMarkers(markers.NewMarker(1, 2)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block5": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block5": tf.Ledger.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6
	{
		tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5", "Block0")), models.WithPayload(tf.Ledger.CreateTransaction("TX5", 1, "TX0.2")))
		tf.BlockDAG.IssueBlocks("Block6")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6": markers.NewMarkers(markers.NewMarker(1, 3)),
		}))

		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6": tf.Ledger.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6.3
	{
		tf.BlockDAG.CreateBlock("Block6.3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block6")))
		tf.BlockDAG.IssueBlocks("Block6.3")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6.3": markers.NewMarkers(markers.NewMarker(1, 4)),
		}))

		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6.3": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6.3": tf.Ledger.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block6.6
	{
		tf.BlockDAG.CreateBlock("Block6.6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block6")))
		tf.BlockDAG.IssueBlocks("Block6.6")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block6.6": markers.NewMarkers(markers.NewMarker(1, 3)),
		}))

		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block6.6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6.6": tf.Ledger.TransactionIDs("TX4"),
		}))
	}

	// ISSUE Block7
	{
		tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5", "Block0")), models.WithPayload(tf.Ledger.CreateTransaction("TX6", 1, "TX0.2")))
		tf.BlockDAG.IssueBlocks("Block7")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7": markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(0, 2)),
		}))

		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7": {tf.Ledger.TransactionIDs("TX6"), utxo.NewTransactionIDs()},
		}))

		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block6":   tf.Ledger.TransactionIDs("TX4", "TX5"),
			"Block6.3": tf.Ledger.TransactionIDs("TX4", "TX5"),
			"Block6.6": tf.Ledger.TransactionIDs("TX4", "TX5"),
			"Block7":   tf.Ledger.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block7.3
	{
		tf.BlockDAG.CreateBlock("Block7.3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7")))
		tf.BlockDAG.IssueBlocks("Block7.3")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7.3": markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(0, 2)),
		}))

		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7.3": {tf.Ledger.TransactionIDs("TX6"), utxo.NewTransactionIDs()},
		}))

		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block7.3": tf.Ledger.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block7.6
	{
		tf.BlockDAG.CreateBlock("Block7.6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7.3")))
		tf.BlockDAG.IssueBlocks("Block7.6")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block7.6": markers.NewMarkers(markers.NewMarker(2, 3)),
		}))

		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block7.6": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))

		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block7.6": tf.Ledger.TransactionIDs("TX4", "TX6"),
		}))
	}

	// ISSUE Block8
	{
		tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithPayload(tf.Ledger.CreateTransaction("TX2", 1, "TX0.1")))
		tf.BlockDAG.IssueBlocks("Block8")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block8": markers.NewMarkers(markers.NewMarker(0, 5)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block3": {tf.Ledger.TransactionIDs("TX3"), utxo.NewTransactionIDs()},
			"Block4": {tf.Ledger.TransactionIDs("TX3", "TX4"), utxo.NewTransactionIDs()},
			"Block8": {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block3":   tf.Ledger.TransactionIDs("TX3"),
			"Block4":   tf.Ledger.TransactionIDs("TX3", "TX4"),
			"Block5":   tf.Ledger.TransactionIDs("TX3", "TX4"),
			"Block6":   tf.Ledger.TransactionIDs("TX3", "TX4", "TX5"),
			"Block6.3": tf.Ledger.TransactionIDs("TX3", "TX4", "TX5"),
			"Block6.6": tf.Ledger.TransactionIDs("TX3", "TX4", "TX5"),
			"Block7":   tf.Ledger.TransactionIDs("TX3", "TX4", "TX6"),
			"Block7.3": tf.Ledger.TransactionIDs("TX3", "TX4", "TX6"),
			"Block7.6": tf.Ledger.TransactionIDs("TX3", "TX4", "TX6"),
			"Block8":   tf.Ledger.TransactionIDs("TX2", "TX1"),
		}))
	}
}

func TestFutureConePropagation(t *testing.T) {
	t.Skip("Skip until we propagate conflicts through Markers again")
	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	tf.BlockDAG.CreateBlock("Block0", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")))

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block0")), models.WithPayload(tf.Ledger.CreateTransaction("TX1", 1, "Genesis")))
	tf.BlockDAG.CreateBlock("Block1*", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block0")), models.WithPayload(tf.Ledger.CreateTransaction("TX1*", 1, "Genesis")))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX2", 1, "TX1.0")))
	tf.BlockDAG.CreateBlock("Block2*", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithPayload(tf.Ledger.CreateTransaction("TX2*", 1, "TX1.0")))

	// these are all liking TX1* -> should not have TX1 and TX2
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block1*"))) // propagation via markers
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block1*"))) // check correct booking (later)
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block1*"))) // propagation via individually mapped blocks
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")))                                                                  // propagation via individually mapped blocks

	// these are liking TX1 -> should have TX2 too
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")))                                                                 // propagation via individually mapped blocks
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block1"))) // propagation via marker
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block1"))) // propagation via marker

	markersMap := make(map[string]*markers.Markers)
	metadataDiffConflictIDs := make(map[string][]utxo.TransactionIDs)
	conflictIDs := make(map[string]utxo.TransactionIDs)

	{
		tf.BlockDAG.IssueBlocks("Block0")
		tf.BlockDAG.IssueBlocks("Block1")
		tf.BlockDAG.IssueBlocks("Block1*")
		tf.BlockDAG.IssueBlocks("Block2")
		tf.BlockDAG.IssueBlocks("Block3")
		tf.BlockDAG.IssueBlocks("Block5", "Block6")
		tf.BlockDAG.IssueBlocks("Block7", "Block8", "Block9")

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
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block1":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1*": {tf.Ledger.TransactionIDs("TX1*"), utxo.NewTransactionIDs()},
			"Block2":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block3":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block5":  {tf.Ledger.TransactionIDs("TX1*"), tf.Ledger.TransactionIDs("TX1")},
			"Block6":  {tf.Ledger.TransactionIDs("TX1*"), tf.Ledger.TransactionIDs("TX1")},
			"Block7":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block8":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block9":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block1":  tf.Ledger.TransactionIDs("TX1"),
			"Block1*": tf.Ledger.TransactionIDs("TX1*"),
			"Block2":  tf.Ledger.TransactionIDs("TX1"),
			"Block3":  tf.Ledger.TransactionIDs("TX1*"),
			"Block5":  tf.Ledger.TransactionIDs("TX1*"),
			"Block6":  tf.Ledger.TransactionIDs("TX1*"),
			"Block7":  tf.Ledger.TransactionIDs("TX1"),
			"Block8":  tf.Ledger.TransactionIDs("TX1"),
			"Block9":  tf.Ledger.TransactionIDs("TX1"),
		}))
	}

	// Verify correct propagation of TX2 to its future cone.
	{
		tf.BlockDAG.IssueBlocks("Block2*")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block2*": markers.NewMarkers(markers.NewMarker(0, 2)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block2*": {tf.Ledger.TransactionIDs("TX2*"), utxo.NewTransactionIDs()},
			"Block5":  {tf.Ledger.TransactionIDs("TX1*"), tf.Ledger.TransactionIDs("TX1")},
			"Block6":  {tf.Ledger.TransactionIDs("TX1*"), tf.Ledger.TransactionIDs("TX1")},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block2":  tf.Ledger.TransactionIDs("TX1", "TX2"),
			"Block2*": tf.Ledger.TransactionIDs("TX1", "TX2*"),
			"Block3":  tf.Ledger.TransactionIDs("TX1*"), // does not change because of marker mapping
			"Block5":  tf.Ledger.TransactionIDs("TX1*"), // does not change because of additional diff entry
			"Block6":  tf.Ledger.TransactionIDs("TX1*"), // does not change because of additional diff entry
			"Block7":  tf.Ledger.TransactionIDs("TX1", "TX2"),
			"Block8":  tf.Ledger.TransactionIDs("TX1", "TX2"),
			"Block9":  tf.Ledger.TransactionIDs("TX1", "TX2"),
		}))
	}

	// Check that inheritance works as expected when booking Block4.
	{
		tf.BlockDAG.IssueBlocks("Block4")

		tf.CheckMarkers(lo.MergeMaps(markersMap, map[string]*markers.Markers{
			"Block4": markers.NewMarkers(markers.NewMarker(0, 3)),
		}))
		tf.CheckBlockMetadataDiffConflictIDs(lo.MergeMaps(metadataDiffConflictIDs, map[string][]utxo.TransactionIDs{
			"Block4": {tf.Ledger.TransactionIDs("TX1*"), tf.Ledger.TransactionIDs("TX1", "TX2")},
		}))
		tf.CheckConflictIDs(lo.MergeMaps(conflictIDs, map[string]utxo.TransactionIDs{
			"Block4": tf.Ledger.TransactionIDs("TX1*"),
		}))
	}
}

func TestWeakParent(t *testing.T) {
	t.Skip("Skip until we propagate conflicts through Markers again")
	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	tf.BlockDAG.CreateBlock("Block0", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")))

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block0")), models.WithPayload(tf.Ledger.CreateTransaction("TX1", 1, "Genesis")))
	tf.BlockDAG.CreateBlock("Block1*", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block0")), models.WithPayload(tf.Ledger.CreateTransaction("TX1*", 1, "Genesis")))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block0")), models.WithWeakParents(tf.BlockDAG.BlockIDs("Block1")))

	{
		tf.BlockDAG.IssueBlocks("Block0")
		tf.BlockDAG.IssueBlocks("Block1")
		tf.BlockDAG.IssueBlocks("Block1*")
		tf.BlockDAG.IssueBlocks("Block2")

		tf.CheckBlockMetadataDiffConflictIDs(map[string][]utxo.TransactionIDs{
			"Block0":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1":  {utxo.NewTransactionIDs(), utxo.NewTransactionIDs()},
			"Block1*": {tf.Ledger.TransactionIDs("TX1*"), utxo.NewTransactionIDs()},
			"Block2":  {tf.Ledger.TransactionIDs("TX1"), utxo.NewTransactionIDs()},
		})
		tf.CheckConflictIDs(map[string]utxo.TransactionIDs{
			"Block0":  tf.Ledger.TransactionIDs(),
			"Block1":  tf.Ledger.TransactionIDs("TX1"),
			"Block1*": tf.Ledger.TransactionIDs("TX1*"),
			"Block2":  tf.Ledger.TransactionIDs("TX1"),
		})
	}
}

func TestMultiThreadedBookingAndForkingParallel(t *testing.T) {
	const layersNum = 127
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	// Create base-layer outputs to double-spend
	tf.BlockDAG.CreateBlock("Block.G", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("G", layersNum, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block.G")

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
				tf.BlockDAG.CreateBlock(blkName, models.WithStrongParents(tf.BlockDAG.BlockIDs(strongParents...)), models.WithPayload(tf.Ledger.CreateTransaction(txAlias, 1, input)))
			} else {
				tf.BlockDAG.CreateBlock(blkName, models.WithStrongParents(tf.BlockDAG.BlockIDs(strongParents...)))
			}

			blks = append(blks, blkName)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < len(blks); i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(int(50 * 1000 * rand.Float32())))
			tf.BlockDAG.IssueBlocks(blks[i])
			wg.Done()
		}(i)
	}

	wg.Wait()
	workers.WaitChildren()

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

			expectedConflicts[blkName] = tf.Ledger.TransactionIDs(conflicts...)
		}
	}

	tf.CheckConflictIDs(expectedConflicts)
}

func TestMultiThreadedBookingAndForkingNested(t *testing.T) {
	const layersNum = 50
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	// Create base-layer outputs to double-spend
	tf.BlockDAG.CreateBlock("Block.G", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithPayload(tf.Ledger.CreateTransaction("G", widthSize, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block.G")

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
			tf.BlockDAG.CreateBlock(blkName, models.WithStrongParents(tf.BlockDAG.BlockIDs(strongParents...)), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs(likeParents...)), models.WithPayload(tf.Ledger.CreateTransaction(txAlias, 1, input)))

			blks = append(blks, blkName)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < len(blks); i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(int(50 * 1000 * rand.Float32())))
			tf.BlockDAG.IssueBlocks(blks[i])
			wg.Done()
		}(i)
	}
	wg.Wait()
	workers.WaitChildren()

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

			expectedConflicts[blkName] = tf.Ledger.TransactionIDs(conflicts...)
		}
	}

	tf.CheckNormalizedConflictIDsContained(expectedConflicts)
}

// This test creates two chains of blocks from the genesis (1 block per slot in each chain). The first chain is solid, the second chain is not.
// When pruning the BlockDAG, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func Test_Prune(t *testing.T) {
	const slotCount = 100

	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	// create a helper function that creates the blocks
	createNewBlock := func(idx slot.Index, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 1 {
			fmt.Println("Creating genesis block")

			return tf.BlockDAG.CreateBlock(
				alias,
				models.WithIssuingTime(tf.BlockDAG.SlotTimeProvider().GenesisTime()),
				models.WithPayload(tf.Ledger.CreateTransaction(alias, 1, "Genesis")),
			), alias
		}
		parentAlias := fmt.Sprintf("blk%s-%d", prefix, idx-1)
		return tf.BlockDAG.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockDAG.BlockIDs(parentAlias)),
			models.WithIssuingTime(tf.BlockDAG.SlotTimeProvider().StartTime(idx)),
			models.WithPayload(tf.Ledger.CreateTransaction(alias, 1, fmt.Sprintf("%s.0", parentAlias))),
		), alias
	}

	require.EqualValues(t, 0, tf.BlockDAG.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().LastEvictedSlot(), "maxDroppedSlot should be 0")

	expectedInvalid := make(map[string]bool, slotCount)
	expectedBooked := make(map[string]bool, slotCount)

	// Attach solid blocks
	for i := slot.Index(1); i <= slotCount; i++ {
		block, alias := createNewBlock(i, "")

		_, wasAttached, err := tf.BlockDAG.Instance.Attach(block)
		require.True(t, wasAttached, "block should be attached")
		require.NoError(t, err, "should not be able to attach a block after shutdown")

		if i >= slotCount/4 {
			expectedInvalid[alias] = false
			expectedBooked[alias] = true
		}
	}

	_, wasAttached, err := tf.BlockDAG.Instance.Attach(tf.BlockDAG.CreateBlock(
		"blk-1-reattachment",
		models.WithStrongParents(tf.BlockDAG.BlockIDs(fmt.Sprintf("blk-%d", slotCount))),
		models.WithIssuingTime(tf.BlockDAG.SlotTimeProvider().StartTime(slotCount)),
		models.WithPayload(tf.Ledger.Transaction("blk-1")),
	))
	require.True(t, wasAttached, "block should be attached")
	require.NoError(t, err, "should not be able to attach a block after shutdown")

	workers.WaitChildren()

	tf.AssertBookedCount(slotCount+1, "should have all solid blocks")

	validateState(tf, 0, slotCount)

	tf.BlockDAG.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().EvictUntil(slotCount / 4)
	workers.WaitChildren()

	require.EqualValues(t, slotCount/4, tf.BlockDAG.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().LastEvictedSlot(), "maxDroppedSlot of booker should be slotCount/4")

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	tf.BlockDAG.AssertInvalidCount(0, "should have invalid blocks")

	tf.BlockDAG.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().EvictUntil(slotCount / 10)
	workers.WaitChildren()

	require.EqualValues(t, slotCount/4, tf.BlockDAG.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().LastEvictedSlot(), "maxDroppedSlot of booker should be slotCount/4")

	tf.BlockDAG.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().EvictUntil(slotCount / 2)
	workers.WaitChildren()

	require.EqualValues(t, slotCount/2, tf.BlockDAG.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().LastEvictedSlot(), "maxDroppedSlot of booker should be slotCount/2")

	validateState(tf, slotCount/2, slotCount)

	_, wasAttached, err = tf.BlockDAG.Instance.Attach(tf.BlockDAG.CreateBlock(
		"blk-0.5",
		models.WithStrongParents(tf.BlockDAG.BlockIDs(fmt.Sprintf("blk-%d", slotCount))),
		models.WithIssuingTime(tf.BlockDAG.SlotTimeProvider().GenesisTime()),
	))
	workers.WaitChildren()

	require.False(t, wasAttached, "block should not be attached")
	require.Error(t, err, "should not be able to attach a block after eviction of a slot")
}

func validateState(tf *booker.TestFramework, maxPrunedSlot, slotCount int) {
	for i := maxPrunedSlot + 1; i <= slotCount; i++ {
		alias := fmt.Sprintf("blk-%d", i)

		_, exists := tf.Instance.Block(tf.BlockDAG.Block(alias).ID())
		require.True(tf.Test, exists, "block should be in the BlockDAG")
		if i == 1 {
			blocks := tf.Instance.GetAllAttachments(tf.Ledger.Transaction(alias).ID())
			require.Equal(tf.Test, 2, blocks.Size(), "transaction blk-0 should have 2 attachments")
		} else {
			blocks := tf.Instance.GetAllAttachments(tf.Ledger.Transaction(alias).ID())
			require.Equal(tf.Test, 1, blocks.Size(), "transaction should have 1 attachment")
		}
	}

	for i := 1; i <= maxPrunedSlot; i++ {
		alias := fmt.Sprintf("blk-%d", i)
		_, exists := tf.Instance.Block(tf.BlockDAG.Block(alias).ID())
		require.False(tf.Test, exists, "block should not be in the BlockDAG")
		if i == 1 {
			blocks := tf.Instance.GetAllAttachments(tf.Ledger.Transaction(alias).ID())
			require.Equal(tf.Test, 1, blocks.Size(), "transaction should have 1 attachment")
		} else {
			blocks := tf.Instance.GetAllAttachments(tf.Ledger.Transaction(alias).ID())
			require.Equal(tf.Test, 0, blocks.Size(), "transaction should have no attachments")
		}
	}
}

func Test_BlockInvalid(t *testing.T) {
	workers := workerpool.NewGroup(t.Name())
	tf := markerbooker.NewDefaultTestFramework(t, workers.CreateGroup("BookerTestFramework"), realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger")))

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block1")))
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block2")))
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis", "Block1")))
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1", "Block2")))
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2", "Block5")))
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block5")))
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block5")))
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4", "Block6")))

	tf.BlockDAG.IssueBlocks("Block1")
	tf.BlockDAG.IssueBlocks("Block3")
	tf.BlockDAG.IssueBlocks("Block4")
	tf.BlockDAG.IssueBlocks("Block5")
	tf.BlockDAG.IssueBlocks("Block6")
	tf.BlockDAG.IssueBlocks("Block7")
	tf.BlockDAG.IssueBlocks("Block8")

	tf.BlockDAG.IssueBlocks("Block2")

	tf.BlockDAG.IssueBlocks("Block9")

	tf.BlockDAG.AssertInvalidCount(7, "8 blocks should be invalid")
	tf.AssertBookedCount(2, "1 block should be booked")
	tf.BlockDAG.AssertInvalid(map[string]bool{
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

	tf.BlockDAG.AssertSolid(map[string]bool{
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

func TestOTV_Track(t *testing.T) {
	// TODO: extend this test to cover the following cases:
	//  - when forking there is already a vote with higher power that should not be migrated
	//  - a voter that supports a marker does not support all the forked conflict's parents
	//  - test issuing votes out of order, votes have same time (possibly separate test case)

	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())

	storageInstance := blockdag.NewTestStorage(t, workers)
	evictionState := eviction.NewState(storageInstance)

	slotTimeProviderFunc := func() *slot.TimeProvider {
		return slot.NewTimeProvider(time.Now().Unix(), 10)
	}
	memPool := realitiesledger.NewTestLedger(t, workers.CreateGroup("RealitiesLedger"))

	validators := sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB()))

	markerBooker := markerbooker.New(workers.CreateGroup("Booker"),
		evictionState,
		memPool,
		validators,
		slotTimeProviderFunc,
		markerbooker.WithMarkerManagerOptions(
			markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](
				markers.WithMaxPastMarkerDistance(3),
			),
		),
	)
	blockDAG := inmemoryblockdag.New(workers.CreateGroup("BlockDAG"), evictionState, slotTimeProviderFunc, blockdag.DefaultCommitmentFunc)

	markerBooker.Initialize(blockDAG)

	tf := booker.NewTestFramework(t, workers.CreateGroup("BookerTestFramework"), markerBooker, blockDAG, memPool, validators, slotTimeProviderFunc)

	tf.VirtualVoting.CreateIdentity("A", 30)
	tf.VirtualVoting.CreateIdentity("B", 15)
	tf.VirtualVoting.CreateIdentity("C", 25)
	tf.VirtualVoting.CreateIdentity("D", 20)
	tf.VirtualVoting.CreateIdentity("E", 10)

	initialMarkerVotes := make(map[markers.Marker]*advancedset.AdvancedSet[identity.ID])
	initialConflictVotes := make(map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID])

	// ISSUE Block1
	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block1")

	tf.AssertBlockTracked(1)

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.VirtualVoting.Votes.ValidatorsSet("A"),
	}))

	// ISSUE Block2
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block2")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.VirtualVoting.Votes.ValidatorsSet("A", "B"),
		markers.NewMarker(0, 2): tf.VirtualVoting.Votes.ValidatorsSet("B"),
	}))

	// ISSUE Block3
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block3")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C"),
		markers.NewMarker(0, 2): tf.VirtualVoting.Votes.ValidatorsSet("B", "C"),
		markers.NewMarker(0, 3): tf.VirtualVoting.Votes.ValidatorsSet("C"),
	}))

	// ISSUE Block4
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithIssuer(tf.VirtualVoting.Identity("D").PublicKey()))

	tf.BlockDAG.IssueBlocks("Block4")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.VirtualVoting.Votes.ValidatorsSet("B", "C", "D"),
		markers.NewMarker(0, 3): tf.VirtualVoting.Votes.ValidatorsSet("C", "D"),
		markers.NewMarker(0, 4): tf.VirtualVoting.Votes.ValidatorsSet("D"),
	}))
	// ISSUE Block5
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()),
		models.WithPayload(tf.Ledger.CreateTransaction("Tx1", 1, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block5")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 3): tf.VirtualVoting.Votes.ValidatorsSet("A", "C", "D"),
		markers.NewMarker(0, 4): tf.VirtualVoting.Votes.ValidatorsSet("A", "D"),
		markers.NewMarker(0, 5): tf.VirtualVoting.Votes.ValidatorsSet("A"),
	}))

	// ISSUE Block6
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("E").PublicKey()),
		models.WithPayload(tf.Ledger.CreateTransaction("Tx2", 1, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block6")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 2): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 3): tf.VirtualVoting.Votes.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.VirtualVoting.Votes.ValidatorsSet("A", "D", "E"),
		markers.NewMarker(0, 5): tf.VirtualVoting.Votes.ValidatorsSet("A"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("E"),
	}))

	// ISSUE Block7
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()), models.WithPayload(tf.Ledger.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.BlockDAG.IssueBlocks("Block7")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 4): tf.VirtualVoting.Votes.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(0, 5): tf.VirtualVoting.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 6): tf.VirtualVoting.Votes.ValidatorsSet("C"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A", "C"),
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("E"),
	}))

	// ISSUE Block7.1
	tf.BlockDAG.CreateBlock("Block7.1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.BlockDAG.IssueBlocks("Block7.1")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 6): tf.VirtualVoting.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 7): tf.VirtualVoting.Votes.ValidatorsSet("A"),
	}))

	// ISSUE Block7.2
	tf.BlockDAG.CreateBlock("Block7.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7.1")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.BlockDAG.IssueBlocks("Block7.2")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 7): tf.VirtualVoting.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 8): tf.VirtualVoting.Votes.ValidatorsSet("C"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("E", "C"),
	}))

	// ISSUE Block8
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.VirtualVoting.Identity("D").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block8")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("C", "D", "E"),
	}))

	// ISSUE Block9
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block8")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block9")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(1, 5): tf.VirtualVoting.Votes.ValidatorsSet("A"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{}))

	// ISSUE Block10
	tf.BlockDAG.CreateBlock("Block10", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block9")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block10")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 3): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(1, 5): tf.VirtualVoting.Votes.ValidatorsSet("A", "B"),
		markers.NewMarker(1, 6): tf.VirtualVoting.Votes.ValidatorsSet("B"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("B", "C", "D", "E"),
	}))

	// ISSUE Block11
	tf.BlockDAG.CreateBlock("Block11", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()), models.WithPayload(tf.Ledger.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.BlockDAG.IssueBlocks("Block11")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx3").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx4").ID(): tf.VirtualVoting.Votes.ValidatorsSet(),
	}))

	// ISSUE Block12
	tf.BlockDAG.CreateBlock("Block12", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block11")), models.WithIssuer(tf.VirtualVoting.Identity("D").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block12")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.VirtualVoting.Votes.ValidatorsSet("A", "C", "D"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A", "D"),
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("B", "C", "E"),
		tf.Ledger.Transaction("Tx4").ID(): tf.VirtualVoting.Votes.ValidatorsSet("D"),
	}))

	// ISSUE Block13
	tf.BlockDAG.CreateBlock("Block13", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block12")), models.WithIssuer(tf.VirtualVoting.Identity("E").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block13")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.VirtualVoting.Votes.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.VirtualVoting.Votes.ValidatorsSet("E"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A", "D", "E"),
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("B", "C"),
		tf.Ledger.Transaction("Tx4").ID(): tf.VirtualVoting.Votes.ValidatorsSet("D", "E"),
	}))

	// ISSUE Block14
	tf.BlockDAG.CreateBlock("Block14", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block13")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block14")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.VirtualVoting.Votes.ValidatorsSet("B", "E"),
		markers.NewMarker(2, 7): tf.VirtualVoting.Votes.ValidatorsSet("B"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "D", "E"),
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet("C"),
		tf.Ledger.Transaction("Tx4").ID(): tf.VirtualVoting.Votes.ValidatorsSet("B", "D", "E"),
	}))

	// ISSUE Block15
	tf.BlockDAG.CreateBlock("Block15", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block14")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.BlockDAG.IssueBlocks("Block15")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "E"),
		markers.NewMarker(2, 7): tf.VirtualVoting.Votes.ValidatorsSet("A", "B"),
		markers.NewMarker(2, 8): tf.VirtualVoting.Votes.ValidatorsSet("A"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx3").ID(): tf.VirtualVoting.Votes.ValidatorsSet(),
		tf.Ledger.Transaction("Tx4").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "D", "E"),
	}))

	// ISSUE Block16
	tf.BlockDAG.CreateBlock("Block16", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block15")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.BlockDAG.IssueBlocks("Block16")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(2, 6): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "E"),
		markers.NewMarker(2, 7): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C"),
		markers.NewMarker(2, 8): tf.VirtualVoting.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(2, 9): tf.VirtualVoting.Votes.ValidatorsSet("C"),
	}))

	tf.VirtualVoting.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		tf.Ledger.Transaction("Tx2").ID(): tf.VirtualVoting.Votes.ValidatorsSet(),
		tf.Ledger.Transaction("Tx4").ID(): tf.VirtualVoting.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
	}))
}
