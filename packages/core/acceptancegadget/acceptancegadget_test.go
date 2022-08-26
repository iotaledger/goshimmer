package acceptancegadget

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"

	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

func TestAcceptanceGadget_update_conflictsStepwise(t *testing.T) {
	// TODO: extend this test to cover conflicts

	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithAcceptanceGadgetOptions(WithMarkerAcceptanceThreshold(0.5)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(booker.WithSequenceManagerOptions(markers.WithMaxPastMarkerDistance(3))))))
	tf.CreateIdentity("A", validator.WithWeight(30))
	tf.CreateIdentity("B", validator.WithWeight(15))
	tf.CreateIdentity("C", validator.WithWeight(25))
	tf.CreateIdentity("D", validator.WithWeight(20))
	tf.CreateIdentity("E", validator.WithWeight(10))

	initialAcceptedBlocks := make(map[string]bool)
	initialAcceptedMarkers := make(map[markers.Marker]bool)

	// ISSUE Block1
	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

	tf.AssertBlockTracked(1)

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block1": false,
	}))
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

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 4): true,
	}))

	// ISSUE Block7
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block5": true,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 5): true,
	}))

	// ISSUE Block7.1
	tf.CreateBlock("Block7.1", models.WithStrongParents(tf.BlockIDs("Block7")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.IssueBlocks("Block7.1").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block7":   true,
		"Block7.1": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 6): true,
		markers.NewMarker(0, 7): false,
	}))

	// ISSUE Block7.2
	tf.CreateBlock("Block7.2", models.WithStrongParents(tf.BlockIDs("Block7.1")), models.WithLikedInsteadParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.IssueBlocks("Block7.2").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block7.1": true,
		"Block7.2": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(0, 7): true,
	}))

	// ISSUE Block8
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block8": false,
	}))
	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	// ISSUE Block9
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block8")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block9": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 5): false,
	}))

	// ISSUE Block10
	tf.CreateBlock("Block10", models.WithStrongParents(tf.BlockIDs("Block9")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block10").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block10": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(1, 6): false,
	}))

	// ISSUE Block11
	tf.CreateBlock("Block11", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.IssueBlocks("Block11").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block11": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	// ISSUE Block12
	tf.CreateBlock("Block12", models.WithStrongParents(tf.BlockIDs("Block11")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block12").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block12": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{}))

	// ISSUE Block13
	tf.CreateBlock("Block13", models.WithStrongParents(tf.BlockIDs("Block12")), models.WithIssuer(tf.Identity("E").PublicKey()))
	tf.IssueBlocks("Block13").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block13": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): false,
	}))

	// ISSUE Block14
	tf.CreateBlock("Block14", models.WithStrongParents(tf.BlockIDs("Block13")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block14").WaitUntilAllTasksProcessed()

	tf.ValidateAcceptedBlocks(lo.MergeMaps(initialAcceptedBlocks, map[string]bool{
		"Block14": false,
	}))

	tf.ValidateAcceptedMarker(lo.MergeMaps(initialAcceptedMarkers, map[markers.Marker]bool{
		markers.NewMarker(2, 6): false,
		markers.NewMarker(2, 7): false,
	}))
}

func TestAcceptanceGadget_update_multipleSequences(t *testing.T) {
	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithAcceptanceGadgetOptions(WithMarkerAcceptanceThreshold(0.66)), WithTangleOptions(tangle.WithBookerOptions(booker.WithMarkerManagerOptions(booker.WithSequenceManagerOptions(markers.WithMaxPastMarkerDistance(3))))))
	tf.CreateIdentity("A", validator.WithWeight(20))
	tf.CreateIdentity("B", validator.WithWeight(30))

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
		tf.AssertBlockAccepted(14)
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
	}
}
