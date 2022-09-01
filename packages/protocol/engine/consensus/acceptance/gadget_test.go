package acceptance

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

func TestGadget_update_conflictsStepwise(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithGadgetOptions(WithConflictAcceptanceThreshold(0.5), WithMarkerAcceptanceThreshold(0.5)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(booker.WithSequenceManagerOptions(markers.WithMaxPastMarkerDistance(3))))))
	tf.Tangle.CreateIdentity("A", validator.WithWeight(30))
	tf.Tangle.CreateIdentity("B", validator.WithWeight(15))
	tf.Tangle.CreateIdentity("C", validator.WithWeight(25))
	tf.Tangle.CreateIdentity("D", validator.WithWeight(20))
	tf.Tangle.CreateIdentity("E", validator.WithWeight(10))

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedConflicts := make(map[string]confirmation.State)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	// ISSUE Block1
	tf.Tangle.CreateBlock("Block1", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

	tf.Tangle.AssertBlockTracked(1)

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block1": false,
	}))
	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 1): false,
	}))

	// ISSUE Block2
	tf.Tangle.CreateBlock("Block2", models.WithStrongParents(tf.Tangle.BlockIDs("Block1")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()))
	tf.Tangle.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
	tf.Tangle.AssertBlockTracked(2)

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block2": false,
	}))
	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 2): false,
	}))

	// ISSUE Block3
	tf.Tangle.CreateBlock("Block3", models.WithStrongParents(tf.Tangle.BlockIDs("Block2")), models.WithIssuer(tf.Tangle.Identity("C").PublicKey()))
	tf.Tangle.IssueBlocks("Block3").WaitUntilAllTasksProcessed()
	tf.Tangle.AssertBlockTracked(3)

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block1": true,
		"Block3": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 1): true,
		markers.NewMarker(0, 3): false,
	}))

	// ISSUE Block4
	tf.Tangle.CreateBlock("Block4", models.WithStrongParents(tf.Tangle.BlockIDs("Block3")), models.WithIssuer(tf.Tangle.Identity("D").PublicKey()))
	tf.Tangle.IssueBlocks("Block4").WaitUntilAllTasksProcessed()
	tf.Tangle.AssertBlockTracked(4)

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block2": true,
		"Block4": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 2): true,
		markers.NewMarker(0, 4): false,
	}))

	// ISSUE Block5
	tf.Tangle.CreateBlock("Block5", models.WithStrongParents(tf.Tangle.BlockIDs("Block4")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()),
		models.WithPayload(tf.Tangle.CreateTransaction("Tx1", 1, "Genesis")))
	tf.Tangle.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block3": true,
		"Block5": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 3): true,
		markers.NewMarker(0, 5): false,
	}))

	// ISSUE Block6
	tf.Tangle.CreateBlock("Block6", models.WithStrongParents(tf.Tangle.BlockIDs("Block4")), models.WithIssuer(tf.Tangle.Identity("E").PublicKey()),
		models.WithPayload(tf.Tangle.CreateTransaction("Tx2", 1, "Genesis")))
	tf.Tangle.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block4": true,
		"Block6": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 4): true,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
		"Tx1": confirmation.Pending,
		"Tx2": confirmation.Pending,
	}))

	// ISSUE Block7
	tf.Tangle.CreateBlock("Block7", models.WithStrongParents(tf.Tangle.BlockIDs("Block5")), models.WithIssuer(tf.Tangle.Identity("C").PublicKey()), models.WithPayload(tf.Tangle.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.Tangle.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block5": true,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 5): true,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block7.1
	tf.Tangle.CreateBlock("Block7.1", models.WithStrongParents(tf.Tangle.BlockIDs("Block7")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.Tangle.IssueBlocks("Block7.1").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block7":   true,
		"Block7.1": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 6): true,
		markers.NewMarker(0, 7): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block7.2
	tf.Tangle.CreateBlock("Block7.2", models.WithStrongParents(tf.Tangle.BlockIDs("Block7.1")), models.WithLikedInsteadParents(tf.Tangle.BlockIDs("Block6")), models.WithIssuer(tf.Tangle.Identity("C").PublicKey()))
	tf.Tangle.IssueBlocks("Block7.2").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block7.1": true,
		"Block7.2": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 7): true,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block8
	tf.Tangle.CreateBlock("Block8", models.WithStrongParents(tf.Tangle.BlockIDs("Block6")), models.WithIssuer(tf.Tangle.Identity("D").PublicKey()))
	tf.Tangle.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block8": false,
	}))
	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block9
	tf.Tangle.CreateBlock("Block9", models.WithStrongParents(tf.Tangle.BlockIDs("Block8")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block9": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 5): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block10
	tf.Tangle.CreateBlock("Block10", models.WithStrongParents(tf.Tangle.BlockIDs("Block9")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()))
	tf.Tangle.IssueBlocks("Block10").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block10": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 6): false,
	}))

	// ISSUE Block11
	tf.Tangle.CreateBlock("Block11", models.WithStrongParents(tf.Tangle.BlockIDs("Block5")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()), models.WithPayload(tf.Tangle.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.Tangle.IssueBlocks("Block11").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block11": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
		"Tx3": confirmation.Pending,
		"Tx4": confirmation.Pending,
	}))

	// ISSUE Block12
	tf.Tangle.CreateBlock("Block12", models.WithStrongParents(tf.Tangle.BlockIDs("Block11")), models.WithIssuer(tf.Tangle.Identity("D").PublicKey()))
	tf.Tangle.IssueBlocks("Block12").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block12": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block13
	tf.Tangle.CreateBlock("Block13", models.WithStrongParents(tf.Tangle.BlockIDs("Block12")), models.WithIssuer(tf.Tangle.Identity("E").PublicKey()))
	tf.Tangle.IssueBlocks("Block13").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block13": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block14
	tf.Tangle.CreateBlock("Block14", models.WithStrongParents(tf.Tangle.BlockIDs("Block13")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()))
	tf.Tangle.IssueBlocks("Block14").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block14": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): false,
		markers.NewMarker(2, 7): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block15
	tf.Tangle.CreateBlock("Block15", models.WithStrongParents(tf.Tangle.BlockIDs("Block14")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.Tangle.IssueBlocks("Block15").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block11": true,
		"Block12": true,
		"Block13": true,
		"Block15": false,
	}))

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
	tf.Tangle.CreateBlock("Block16", models.WithStrongParents(tf.Tangle.BlockIDs("Block15")), models.WithIssuer(tf.Tangle.Identity("C").PublicKey()))
	tf.Tangle.IssueBlocks("Block16").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block14": true,
		"Block15": true,
		"Block16": false,
	}))

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
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithGadgetOptions(WithMarkerAcceptanceThreshold(0.66)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(booker.WithSequenceManagerOptions(markers.WithMaxPastMarkerDistance(3))))))
	tf.Tangle.CreateIdentity("A", validator.WithWeight(20))
	tf.Tangle.CreateIdentity("B", validator.WithWeight(30))

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	tf.Tangle.CreateBlock("Block1", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block2", models.WithStrongParents(tf.Tangle.BlockIDs("Block1")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block3", models.WithStrongParents(tf.Tangle.BlockIDs("Block2")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block4", models.WithStrongParents(tf.Tangle.BlockIDs("Block3")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block5", models.WithStrongParents(tf.Tangle.BlockIDs("Block4")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))

	tf.Tangle.CreateBlock("Block6", models.WithStrongParents(tf.Tangle.BlockIDs("Block4")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block7", models.WithStrongParents(tf.Tangle.BlockIDs("Block6")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block8", models.WithStrongParents(tf.Tangle.BlockIDs("Block7")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block9", models.WithStrongParents(tf.Tangle.BlockIDs("Block8")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block10", models.WithStrongParents(tf.Tangle.BlockIDs("Block9")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))

	tf.Tangle.CreateBlock("Block11", models.WithStrongParents(tf.Tangle.BlockIDs("Block4")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block12", models.WithStrongParents(tf.Tangle.BlockIDs("Block11")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block13", models.WithStrongParents(tf.Tangle.BlockIDs("Block12")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block14", models.WithStrongParents(tf.Tangle.BlockIDs("Block13")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block15", models.WithStrongParents(tf.Tangle.BlockIDs("Block14")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))

	tf.Tangle.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5").WaitUntilAllTasksProcessed()
	tf.Tangle.IssueBlocks("Block6", "Block7", "Block8", "Block9", "Block10").WaitUntilAllTasksProcessed()
	tf.Tangle.IssueBlocks("Block11", "Block12", "Block13", "Block14", "Block15").WaitUntilAllTasksProcessed()

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
	}

	tf.Tangle.CreateBlock("Block16", models.WithStrongParents(tf.Tangle.BlockIDs("Block15")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()))
	tf.Tangle.IssueBlocks("Block16").WaitUntilAllTasksProcessed()
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
	}

	tf.Tangle.CreateBlock("Block17", models.WithStrongParents(tf.Tangle.BlockIDs("Block10")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()))
	tf.Tangle.IssueBlocks("Block17").WaitUntilAllTasksProcessed()
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
		tf.AssertBlockAccepted(14)
	}

	tf.Tangle.CreateBlock("Block18", models.WithStrongParents(tf.Tangle.BlockIDs("Block5")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()))
	tf.Tangle.IssueBlocks("Block18").WaitUntilAllTasksProcessed()
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
	}
}

func TestGadget_update_reorg(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithGadgetOptions(WithMarkerAcceptanceThreshold(0.66)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(booker.WithSequenceManagerOptions(markers.WithMaxPastMarkerDistance(3))))))
	tf.Tangle.CreateIdentity("A", validator.WithWeight(20))
	tf.Tangle.CreateIdentity("B", validator.WithWeight(30))

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedConflicts := make(map[string]confirmation.State)

	tf.Tangle.CreateBlock("Block1", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()), models.WithPayload(tf.Tangle.CreateTransaction("Tx1", 1, "Genesis")))
	tf.Tangle.CreateBlock("Block2", models.WithStrongParents(tf.Tangle.BlockIDs("Genesis")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()), models.WithPayload(tf.Tangle.CreateTransaction("Tx2", 1, "Genesis")))

	tf.Tangle.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	tf.Tangle.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1": false,
			"Block2": false,
		}))

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
			"Tx1": confirmation.Pending,
			"Tx2": confirmation.Pending,
		}))

		tf.AssertConflictsAccepted(0)
		tf.AssertConflictsRejected(0)
	}

	tf.Tangle.CreateBlock("Block3", models.WithStrongParents(tf.Tangle.BlockIDs("Block1")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()), models.WithPayload(tf.Tangle.CreateTransaction("Tx3", 1, "Tx1.0")))

	tf.Tangle.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1": true,
			"Block3": false,
		}))

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
			"Tx1": confirmation.Accepted,
			"Tx2": confirmation.Rejected,
		}))

		tf.AssertConflictsAccepted(1)
		tf.AssertConflictsRejected(1)
	}

	tf.Tangle.CreateBlock("Block4", models.WithStrongParents(tf.Tangle.BlockIDs("Block2")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))
	tf.Tangle.CreateBlock("Block5", models.WithStrongParents(tf.Tangle.BlockIDs("Block4")), models.WithIssuer(tf.Tangle.Identity("A").PublicKey()))

	tf.Tangle.IssueBlocks("Block4", "Block5").WaitUntilAllTasksProcessed()

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block2": false,
			"Block4": false,
			"Block5": false,
		}))

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

		tf.AssertConflictsAccepted(1)
		tf.AssertConflictsRejected(1)
		tf.AssertReorgs(0)
	}

	tf.Tangle.CreateBlock("Block6", models.WithStrongParents(tf.Tangle.BlockIDs("Block5")), models.WithIssuer(tf.Tangle.Identity("B").PublicKey()))
	tf.Tangle.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block2": true,
			"Block4": true,
			"Block5": true,
			"Block6": false,
		}))

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

		tf.AssertConflictsAccepted(1)
		tf.AssertConflictsRejected(1)
		tf.AssertReorgs(1)
	}
}
