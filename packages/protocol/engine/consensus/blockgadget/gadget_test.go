package blockgadget

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestGadget_update_conflictsStepwise(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t,
		WithGadgetOptions(
			WithConfirmationThreshold(0.5),
			WithConflictAcceptanceThreshold(0.5),
			WithMarkerAcceptanceThreshold(0.5),
		),
		WithTangleOptions(
			tangle.WithBookerOptions(
				booker.WithMarkerManagerOptions(
					markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3)),
				),
			),
		),
	)
	tf.CreateIdentity("A", 30)
	tf.CreateIdentity("B", 15)
	tf.CreateIdentity("C", 25)
	tf.CreateIdentity("D", 20)
	tf.CreateIdentity("E", 10)

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedConflicts := make(map[string]confirmation.State)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	// ISSUE Block1
	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

	tf.AssertBlockTracked(1)

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block1": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 1): false,
	}))

	// ISSUE Block2
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
	tf.AssertBlockTracked(2)

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block2": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 2): false,
	}))

	// ISSUE Block3
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()
	tf.AssertBlockTracked(3)

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
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()
	tf.AssertBlockTracked(4)

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
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()),
		models.WithPayload(tf.CreateTransaction("Tx1", 1, "Genesis")))
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

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
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("E").PublicKey()),
		models.WithPayload(tf.CreateTransaction("Tx2", 1, "Genesis")))
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

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
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block5": true,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 5): true,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block7.1
	tf.CreateBlock("Block7.1", models.WithStrongParents(tf.BlockIDs("Block7")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.IssueBlocks("Block7.1").WaitUntilAllTasksProcessed()

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
	tf.CreateBlock("Block7.2", models.WithStrongParents(tf.BlockIDs("Block7.1")), models.WithLikedInsteadParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.IssueBlocks("Block7.2").WaitUntilAllTasksProcessed()

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
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block8": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block9
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block8")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block9": false,
	}))

	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 5): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block10
	tf.CreateBlock("Block10", models.WithStrongParents(tf.BlockIDs("Block9")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block10").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block10": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 6): false,
	}))

	// ISSUE Block11
	tf.CreateBlock("Block11", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.IssueBlocks("Block11").WaitUntilAllTasksProcessed()

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
	tf.CreateBlock("Block12", models.WithStrongParents(tf.BlockIDs("Block11")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block12").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block12": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block13
	tf.CreateBlock("Block13", models.WithStrongParents(tf.BlockIDs("Block12")), models.WithIssuer(tf.Identity("E").PublicKey()))
	tf.IssueBlocks("Block13").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block13": false,
	}))
	tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): false,
	}))

	tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

	// ISSUE Block14
	tf.CreateBlock("Block14", models.WithStrongParents(tf.BlockIDs("Block13")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block14").WaitUntilAllTasksProcessed()

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
	tf.CreateBlock("Block15", models.WithStrongParents(tf.BlockIDs("Block14")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.IssueBlocks("Block15").WaitUntilAllTasksProcessed()

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
	tf.CreateBlock("Block16", models.WithStrongParents(tf.BlockIDs("Block15")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.IssueBlocks("Block16").WaitUntilAllTasksProcessed()

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

	tf := NewTestFramework(t, WithGadgetOptions(WithMarkerAcceptanceThreshold(0.66), WithConfirmationThreshold(0.66)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3))))))
	tf.CreateIdentity("A", 20)
	tf.CreateIdentity("B", 30)

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()))

	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block7")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block8")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block10", models.WithStrongParents(tf.BlockIDs("Block9")), models.WithIssuer(tf.Identity("A").PublicKey()))

	tf.CreateBlock("Block11", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block12", models.WithStrongParents(tf.BlockIDs("Block11")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block13", models.WithStrongParents(tf.BlockIDs("Block12")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block14", models.WithStrongParents(tf.BlockIDs("Block13")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block15", models.WithStrongParents(tf.BlockIDs("Block14")), models.WithIssuer(tf.Identity("A").PublicKey()))

	tf.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block6", "Block7", "Block8", "Block9", "Block10").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block11", "Block12", "Block13", "Block14", "Block15").WaitUntilAllTasksProcessed()

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

	tf.CreateBlock("Block16", models.WithStrongParents(tf.BlockIDs("Block15")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block16").WaitUntilAllTasksProcessed()
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

	tf.CreateBlock("Block17", models.WithStrongParents(tf.BlockIDs("Block10")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block17").WaitUntilAllTasksProcessed()
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

	tf.CreateBlock("Block18", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block18").WaitUntilAllTasksProcessed()
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
	tf := NewTestFramework(t, WithTotalWeightCallback(func() int64 {
		return 100
	}), WithGadgetOptions(WithMarkerAcceptanceThreshold(0.66), WithConfirmationThreshold(0.66)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3))))))
	tf.CreateIdentity("A", 20)
	tf.CreateIdentity("B", 30)

	initialAcceptedBlocks := make(map[string]bool)
	initialConfirmedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()))

	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block7")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block8")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block10", models.WithStrongParents(tf.BlockIDs("Block9")), models.WithIssuer(tf.Identity("A").PublicKey()))

	tf.CreateBlock("Block11", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block12", models.WithStrongParents(tf.BlockIDs("Block11")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block13", models.WithStrongParents(tf.BlockIDs("Block12")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block14", models.WithStrongParents(tf.BlockIDs("Block13")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block15", models.WithStrongParents(tf.BlockIDs("Block14")), models.WithIssuer(tf.Identity("A").PublicKey()))

	tf.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block6", "Block7", "Block8", "Block9", "Block10").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block11", "Block12", "Block13", "Block14", "Block15").WaitUntilAllTasksProcessed()

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

	tf.CreateBlock("Block16", models.WithStrongParents(tf.BlockIDs("Block15")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block16").WaitUntilAllTasksProcessed()
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

		tf.ValidateConfirmedBlocks(initialConfirmedBlocks)

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

	tf.CreateBlock("Block17", models.WithStrongParents(tf.BlockIDs("Block10")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block17").WaitUntilAllTasksProcessed()
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

	tf.CreateBlock("Block18", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block18").WaitUntilAllTasksProcessed()
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
	tf.CreateIdentity("C", 50)

	tf.CreateBlock("Block19", models.WithStrongParents(tf.BlockIDs("Block15")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.IssueBlocks("Block19").WaitUntilAllTasksProcessed()
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

	tf.CreateBlock("Block20", models.WithStrongParents(tf.BlockIDs("Block10")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.IssueBlocks("Block20").WaitUntilAllTasksProcessed()
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

	tf.CreateBlock("Block21", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.IssueBlocks("Block21").WaitUntilAllTasksProcessed()
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

func TestGadget_update_reorg(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithGadgetOptions(WithMarkerAcceptanceThreshold(0.66)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3))))))
	tf.CreateIdentity("A", 20)
	tf.CreateIdentity("B", 30)

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedConflicts := make(map[string]confirmation.State)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx1", 1, "Genesis")))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("B").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx2", 1, "Genesis")))

	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1": false,
			"Block2": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
			"Tx1": confirmation.Pending,
			"Tx2": confirmation.Pending,
		}))

		tf.AssertConflictsAccepted(0)
		tf.AssertConflictsRejected(0)
	}

	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithIssuer(tf.Identity("B").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx3", 1, "Tx1.0")))

	tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block1": true,
			"Block3": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{
			"Tx1": confirmation.Accepted,
			"Tx2": confirmation.Rejected,
		}))

		tf.AssertConflictsAccepted(1)
		tf.AssertConflictsRejected(1)
	}

	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block2": true,
			"Block4": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

		tf.AssertConflictsAccepted(1)
		tf.AssertConflictsRejected(1)
		tf.AssertReorgs(0)
	}

	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
	{
		tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
			"Block2": true,
			"Block4": true,
			"Block5": false,
		}))
		tf.ValidateConfirmedBlocks(initialAcceptedBlocks)

		tf.ValidateConflictAcceptance(lo.MergeMaps(initialAcceptedConflicts, map[string]confirmation.State{}))

		tf.AssertConflictsAccepted(1)
		tf.AssertConflictsRejected(1)
		tf.AssertReorgs(1)
	}
}

func TestGadget_unorphan(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithGadgetOptions(WithMarkerAcceptanceThreshold(0.66), WithConfirmationThreshold(0.66)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](markers.WithMaxPastMarkerDistance(3))))))
	tf.CreateIdentity("A", 20)
	tf.CreateIdentity("B", 30)

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()))

	tf.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5").WaitUntilAllTasksProcessed()

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
		block := tf.Block(alias)
		tf.BlockDAG.SetOrphaned(block.Block, true)
	}

	{
		tf.AssertOrphanedBlocks(tf.BlockIDs(
			"Block1",
			"Block2",
			"Block3",
			"Block4",
			"Block5",
		))
		tf.AssertOrphanedCount(5)
	}

	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
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

		tf.AssertOrphanedBlocks(tf.BlockIDs())
		tf.AssertOrphanedCount(0)
	}
}
