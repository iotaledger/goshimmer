package tresholdblockgadget_test

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget/tresholdblockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/realitiesledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/testtangle"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, memPool mempool.MemPool, optsGadget ...options.Option[tresholdblockgadget.Gadget]) *blockgadget.TestFramework {
	tangleTF := testtangle.NewDefaultTestFramework(t, workers.CreateGroup("TangleTestFramework"),
		memPool,
		slot.NewTimeProvider(time.Now().Unix(), 10),
		markerbooker.WithMarkerManagerOptions(
			markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3)),
		),
	)

	gadget := tresholdblockgadget.New(
		optsGadget...,
	)

	gadget.Initialize(workers.CreateGroup("BlockGadget"),
		tangleTF.Instance.Booker(),
		tangleTF.Instance.BlockDAG(),
		memPool,
		tangleTF.Instance.(*testtangle.TestTangle).EvictionState(), tangleTF.Instance.(*testtangle.TestTangle).SlotTimeProvider(), tangleTF.Votes.Validators, tangleTF.Votes.Validators.TotalWeight)

	return blockgadget.NewTestFramework(t,
		gadget,
		tangleTF,
	)
}

func TestGadget_update_conflictsStepwise(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())

	tf := NewDefaultTestFramework(t,
		workers.CreateGroup("BlockGadgetTestFramework"),
		realitiesledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
		tresholdblockgadget.WithConfirmationThreshold(0.5),
		tresholdblockgadget.WithConflictAcceptanceThreshold(0.5),
		tresholdblockgadget.WithMarkerAcceptanceThreshold(0.5),
	)

	tf.VirtualVoting.CreateIdentity("A", 30)
	tf.VirtualVoting.CreateIdentity("B", 15)
	tf.VirtualVoting.CreateIdentity("C", 25)
	tf.VirtualVoting.CreateIdentity("D", 20)
	tf.VirtualVoting.CreateIdentity("E", 10)

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedConflicts := make(map[string]confirmation.State)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	// ISSUE Block1
	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block1")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block1": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 1): false,
	}))

	// ISSUE Block2
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block2")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block2": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 2): false,
	}))

	// ISSUE Block3
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block3")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block1": true,
		"Block3": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 1): true,
		markers.NewMarker(0, 3): false,
	}))

	// ISSUE Block4
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithIssuer(tf.VirtualVoting.Identity("D").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block4")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block2": true,
		"Block4": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 2): true,
		markers.NewMarker(0, 4): false,
	}))

	// ISSUE Block5
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()),
		models.WithPayload(tf.MemPool.CreateTransaction("Tx1", 1, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block5")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block3": true,
		"Block5": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 3): true,
		markers.NewMarker(0, 5): false,
	}))

	// ISSUE Block6
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("E").PublicKey()),
		models.WithPayload(tf.MemPool.CreateTransaction("Tx2", 1, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block6")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block4": true,
		"Block6": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 4): true,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
		"Tx1": confirmation.Pending,
		"Tx2": confirmation.Pending,
	}))

	// ISSUE Block7
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()), models.WithPayload(tf.MemPool.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.BlockDAG.IssueBlocks("Block7")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block5": true,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 5): true,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block7.1
	tf.BlockDAG.CreateBlock("Block7.1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.BlockDAG.IssueBlocks("Block7.1")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block7":   true,
		"Block7.1": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 6): true,
		markers.NewMarker(0, 7): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block7.2
	tf.BlockDAG.CreateBlock("Block7.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7.1")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.BlockDAG.IssueBlocks("Block7.2")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block7.1": true,
		"Block7.2": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 7): true,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block8
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.VirtualVoting.Identity("D").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block8")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block8": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block9
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block8")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block9")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block9": false,
	}))

	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 5): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block10
	tf.BlockDAG.CreateBlock("Block10", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block9")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block10")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block10": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 6): false,
	}))

	// ISSUE Block11
	tf.BlockDAG.CreateBlock("Block11", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()), models.WithPayload(tf.MemPool.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.BlockDAG.IssueBlocks("Block11")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block11": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
		"Tx3": confirmation.Pending,
		"Tx4": confirmation.Pending,
	}))

	// ISSUE Block12
	tf.BlockDAG.CreateBlock("Block12", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block11")), models.WithIssuer(tf.VirtualVoting.Identity("D").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block12")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block12": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block13
	tf.BlockDAG.CreateBlock("Block13", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block12")), models.WithIssuer(tf.VirtualVoting.Identity("E").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block13")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block13": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block14
	tf.BlockDAG.CreateBlock("Block14", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block13")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block14")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block14": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): false,
		markers.NewMarker(2, 7): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block15
	tf.BlockDAG.CreateBlock("Block15", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block14")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.BlockDAG.IssueBlocks("Block15")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block11": true,
		"Block12": true,
		"Block13": true,
		"Block15": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): true,
		markers.NewMarker(2, 8): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
		"Tx1": confirmation.Accepted,
		"Tx2": confirmation.Rejected,
		"Tx3": confirmation.Rejected,
		"Tx4": confirmation.Accepted,
	}))

	// ISSUE Block16
	tf.BlockDAG.CreateBlock("Block16", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block15")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.BlockDAG.IssueBlocks("Block16")

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block14": true,
		"Block15": true,
		"Block16": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 7): true,
		markers.NewMarker(2, 8): true,
		markers.NewMarker(2, 9): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	tf.AssertConflictsAccepted(2)
	tf.AssertConflictsRejected(2)
}

func TestGadget_update_multipleSequences(t *testing.T) {
	debug.SetEnabled(false)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())

	tf := NewDefaultTestFramework(t,
		workers.CreateGroup("BlockGadgetTestFramework"),
		realitiesledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
		tresholdblockgadget.WithMarkerAcceptanceThreshold(0.66),
		tresholdblockgadget.WithConfirmationThreshold(0.66),
	)

	tf.VirtualVoting.CreateIdentity("A", 20)
	tf.VirtualVoting.CreateIdentity("B", 30)

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))

	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block8")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block10", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block9")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))

	tf.BlockDAG.CreateBlock("Block11", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block12", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block11")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block13", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block12")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block14", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block13")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block15", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block14")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))

	tf.BlockDAG.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5")
	tf.BlockDAG.IssueBlocks("Block6", "Block7", "Block8", "Block9", "Block10")
	tf.BlockDAG.IssueBlocks("Block11", "Block12", "Block13", "Block14", "Block15")

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1":  false,
			"Block2":  false,
			"Block3":  false,
			"Block4":  false,
			"Block5":  false,
			"Block6":  false,
			"Block7":  false,
			"Block8":  false,
			"Block9":  false,
			"Block10": false,
			"Block11": false,
			"Block12": false,
			"Block13": false,
			"Block14": false,
			"Block15": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 1): false,
			markers.NewMarker(0, 2): false,
			markers.NewMarker(0, 3): false,
			markers.NewMarker(0, 4): false,
			markers.NewMarker(1, 5): false,
			markers.NewMarker(1, 6): false,
			markers.NewMarker(1, 7): false,
			markers.NewMarker(2, 8): false,
			markers.NewMarker(2, 5): false,
			markers.NewMarker(2, 6): false,
			markers.NewMarker(2, 7): false,
		}))
		tf.AssertBlockAccepted(0)
		tf.AssertBlockConfirmed(0)
	}

	tf.BlockDAG.CreateBlock("Block16", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block15")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block16")
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1":  true,
			"Block2":  true,
			"Block3":  true,
			"Block4":  true,
			"Block11": true,
			"Block12": true,
			"Block13": true,
			"Block14": true,
			"Block15": true,
			"Block16": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 1): true,
			markers.NewMarker(0, 2): true,
			markers.NewMarker(0, 3): true,
			markers.NewMarker(0, 4): true,
			markers.NewMarker(2, 5): true,
			markers.NewMarker(2, 6): true,
			markers.NewMarker(2, 7): true,
			markers.NewMarker(2, 8): false,
		}))
		tf.AssertBlockAccepted(9)
		tf.AssertBlockConfirmed(9)
	}

	tf.BlockDAG.CreateBlock("Block17", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block10")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block17")
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block6":  true,
			"Block7":  true,
			"Block8":  true,
			"Block9":  true,
			"Block10": true,
			"Block17": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(1, 5): true,
			markers.NewMarker(1, 6): true,
			markers.NewMarker(1, 7): true,
			markers.NewMarker(1, 8): false,
		}))
		tf.AssertBlockAccepted(14)
		tf.AssertBlockConfirmed(14)
	}

	tf.BlockDAG.CreateBlock("Block18", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block18")
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block5":  true,
			"Block18": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 5): true,
			markers.NewMarker(0, 6): false,
		}))
		tf.AssertBlockAccepted(15)
		tf.AssertBlockConfirmed(15)
	}
}

func TestGadget_update_multipleSequences_onlyAcceptThenConfirm(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())

	tangleTF := testtangle.NewDefaultTestFramework(t, workers.CreateGroup("TangleTestFramework"),
		realitiesledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
		slot.NewTimeProvider(time.Now().Unix(), 10),
		markerbooker.WithMarkerManagerOptions(
			markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3)),
		),
	)

	gadget := tresholdblockgadget.New(
		tresholdblockgadget.WithMarkerAcceptanceThreshold(0.66),
		tresholdblockgadget.WithConfirmationThreshold(0.66),
	)

	gadget.Initialize(workers.CreateGroup("BlockGadget"),
		tangleTF.Instance.Booker(),
		tangleTF.Instance.BlockDAG(),
		tangleTF.Instance.(*testtangle.TestTangle).MemPool(),
		tangleTF.Instance.(*testtangle.TestTangle).EvictionState(),
		tangleTF.Instance.(*testtangle.TestTangle).SlotTimeProvider(),
		tangleTF.Instance.(*testtangle.TestTangle).Validators(),
		func() int64 {
			return 100
		})

	tf := blockgadget.NewTestFramework(t, gadget, tangleTF)

	tf.VirtualVoting.CreateIdentity("A", 20)
	tf.VirtualVoting.CreateIdentity("B", 30)

	initialAcceptedBlocks := make(map[string]bool)
	initialConfirmedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))

	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block8")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block10", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block9")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))

	tf.BlockDAG.CreateBlock("Block11", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block12", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block11")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block13", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block12")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block14", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block13")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block15", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block14")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))

	tf.BlockDAG.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5")
	tf.BlockDAG.IssueBlocks("Block6", "Block7", "Block8", "Block9", "Block10")
	tf.BlockDAG.IssueBlocks("Block11", "Block12", "Block13", "Block14", "Block15")

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1":  false,
			"Block2":  false,
			"Block3":  false,
			"Block4":  false,
			"Block5":  false,
			"Block6":  false,
			"Block7":  false,
			"Block8":  false,
			"Block9":  false,
			"Block10": false,
			"Block11": false,
			"Block12": false,
			"Block13": false,
			"Block14": false,
			"Block15": false,
		}))

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 1): false,
			markers.NewMarker(0, 2): false,
			markers.NewMarker(0, 3): false,
			markers.NewMarker(0, 4): false,
			markers.NewMarker(1, 5): false,
			markers.NewMarker(1, 6): false,
			markers.NewMarker(1, 7): false,
			markers.NewMarker(2, 8): false,
			markers.NewMarker(2, 5): false,
			markers.NewMarker(2, 6): false,
			markers.NewMarker(2, 7): false,
		}))
		tf.AssertBlockAccepted(0)
		tf.AssertBlockConfirmed(0)
	}

	tf.BlockDAG.CreateBlock("Block16", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block15")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block16")
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1":  true,
			"Block2":  true,
			"Block3":  true,
			"Block4":  true,
			"Block11": true,
			"Block12": true,
			"Block13": true,
			"Block14": true,
			"Block15": true,
			"Block16": false,
		}))

		tf.ValidateConfirmedBlocks(lo.MergeMaps(initialConfirmedBlocks, map[string]bool{
			"Block1":  false,
			"Block2":  false,
			"Block3":  false,
			"Block4":  false,
			"Block5":  false,
			"Block6":  false,
			"Block7":  false,
			"Block8":  false,
			"Block9":  false,
			"Block10": false,
			"Block11": false,
			"Block12": false,
			"Block13": false,
			"Block14": false,
			"Block15": false,
		}))

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 1): true,
			markers.NewMarker(0, 2): true,
			markers.NewMarker(0, 3): true,
			markers.NewMarker(0, 4): true,
			markers.NewMarker(2, 5): true,
			markers.NewMarker(2, 6): true,
			markers.NewMarker(2, 7): true,
			markers.NewMarker(2, 8): false,
		}))
		tf.AssertBlockAccepted(9)
		tf.AssertBlockConfirmed(0)
	}

	tf.BlockDAG.CreateBlock("Block17", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block10")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block17")
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block6":  true,
			"Block7":  true,
			"Block8":  true,
			"Block9":  true,
			"Block10": true,
			"Block17": false,
		}))

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(1, 5): true,
			markers.NewMarker(1, 6): true,
			markers.NewMarker(1, 7): true,
			markers.NewMarker(1, 8): false,
		}))
		tf.ValidateConfirmedBlocks(initialConfirmedBlocks)
		tf.AssertBlockAccepted(14)
		tf.AssertBlockConfirmed(0)
	}

	tf.BlockDAG.CreateBlock("Block18", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block18")
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block5":  true,
			"Block18": false,
		}))

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 5): true,
			markers.NewMarker(0, 6): false,
		}))
		tf.AssertBlockAccepted(15)
		tf.AssertBlockConfirmed(0)
	}

	// Add identity to start confirming blocks
	tf.VirtualVoting.CreateIdentity("C", 50)

	tf.BlockDAG.CreateBlock("Block19", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block15")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block19")
	{
		tf.ValidateAcceptedBlocks(initialAcceptedBlocks)
		tf.ValidateAcceptedMarker(initialAcceptedMarkers)
		tf.ValidateConfirmedBlocks(lo.MergeMaps(initialConfirmedBlocks, map[string]bool{
			"Block1":  true,
			"Block2":  true,
			"Block3":  true,
			"Block4":  true,
			"Block11": true,
			"Block12": true,
			"Block13": true,
			"Block14": true,
			"Block15": true,
			"Block16": false,
			"Block19": false,
		}))
		tf.AssertBlockAccepted(15)
		tf.AssertBlockConfirmed(9)
	}

	tf.BlockDAG.CreateBlock("Block20", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block10")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block20")
	{
		tf.ValidateConfirmedBlocks(lo.MergeMaps(initialConfirmedBlocks, map[string]bool{
			"Block6":  true,
			"Block7":  true,
			"Block8":  true,
			"Block9":  true,
			"Block10": true,
			"Block17": false,
			"Block20": false,
		}))

		tf.ValidateAcceptedMarker(initialAcceptedMarkers)
		tf.AssertBlockAccepted(15)
		tf.AssertBlockConfirmed(14)
	}

	tf.BlockDAG.CreateBlock("Block21", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("C").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block21")
	{
		tf.ValidateConfirmedBlocks(lo.MergeMaps(initialConfirmedBlocks, map[string]bool{
			"Block5":  true,
			"Block18": false,
			"Block21": false,
		}))

		tf.ValidateAcceptedMarker(initialAcceptedMarkers)

		tf.AssertBlockAccepted(15)
		tf.AssertBlockConfirmed(15)
	}
}

func TestGadget_unorphan(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())

	tf := NewDefaultTestFramework(t,
		workers.CreateGroup("BlockGadgetTestFramework"),
		realitiesledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
		tresholdblockgadget.WithMarkerAcceptanceThreshold(0.66),
		tresholdblockgadget.WithConfirmationThreshold(0.66),
	)

	tf.VirtualVoting.CreateIdentity("A", 20)
	tf.VirtualVoting.CreateIdentity("B", 30)

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.VirtualVoting.Identity("A").PublicKey()))

	tf.BlockDAG.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5")

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1": false,
			"Block2": false,
			"Block3": false,
			"Block4": false,
			"Block5": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 1): false,
			markers.NewMarker(0, 2): false,
			markers.NewMarker(0, 3): false,
			markers.NewMarker(0, 4): false,
			markers.NewMarker(0, 5): false,
		}))

		tf.AssertBlockAccepted(0)
		tf.AssertBlockConfirmed(0)
	}

	for _, alias := range []string{"Block1", "Block2", "Block3", "Block4", "Block5"} {
		modelsBlock := tf.BlockDAG.Block(alias)
		block, _ := tf.BlockDAG.Instance.Block(modelsBlock.ID())
		tf.BlockDAG.Instance.SetOrphaned(block, true)
	}

	{
		tf.BlockDAG.AssertOrphanedBlocks(tf.BlockDAG.BlockIDs(
			"Block1",
			"Block2",
			"Block3",
			"Block4",
			"Block5",
		))
		tf.BlockDAG.AssertOrphanedCount(5)
	}

	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.VirtualVoting.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block6")
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1": true,
			"Block2": true,
			"Block3": true,
			"Block4": true,
			"Block5": true,
			"Block6": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
			markers.NewMarker(0, 1): true,
			markers.NewMarker(0, 2): true,
			markers.NewMarker(0, 3): true,
			markers.NewMarker(0, 4): true,
			markers.NewMarker(0, 5): true,
			markers.NewMarker(0, 6): false,
		}))
		tf.AssertBlockAccepted(5)
		tf.AssertBlockConfirmed(5)

		tf.BlockDAG.AssertOrphanedBlocks(tf.BlockDAG.BlockIDs())
		tf.BlockDAG.AssertOrphanedCount(0)
	}
}
