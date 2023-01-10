package virtualvoting

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func TestOTV_Track(t *testing.T) {
	// TODO: extend this test to cover the following cases:
	//  - when forking there is already a vote with higher power that should not be migrated
	//  - a voter that supports a marker does not support all the forked conflict's parents
	//  - test issuing votes out of order, votes have same time (possibly separate test case)

	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	tf := NewTestFramework(t, WithBookerOptions(
		booker.WithMarkerManagerOptions(
			markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](
				markers.WithMaxPastMarkerDistance(3),
			),
		),
	))

	tf.CreateIdentity("A", 30)
	tf.CreateIdentity("B", 15)
	tf.CreateIdentity("C", 25)
	tf.CreateIdentity("D", 20)
	tf.CreateIdentity("E", 10)

	initialMarkerVotes := make(map[markers.Marker]*set.AdvancedSet[identity.ID])
	initialConflictVotes := make(map[utxo.TransactionID]*set.AdvancedSet[identity.ID])

	// ISSUE Block1
	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

	tf.AssertBlockTracked(1)

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.ValidatorsSet("A"),
	}))

	// ISSUE Block2
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.ValidatorsSet("A", "B"),
		markers.NewMarker(0, 2): tf.ValidatorsSet("B"),
	}))

	// ISSUE Block3
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.ValidatorsSet("A", "B", "C"),
		markers.NewMarker(0, 2): tf.ValidatorsSet("B", "C"),
		markers.NewMarker(0, 3): tf.ValidatorsSet("C"),
	}))

	// ISSUE Block4
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithIssuer(tf.Identity("D").PublicKey()))

	tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.ValidatorsSet("B", "C", "D"),
		markers.NewMarker(0, 3): tf.ValidatorsSet("C", "D"),
		markers.NewMarker(0, 4): tf.ValidatorsSet("D"),
	}))
	// ISSUE Block5
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()),
		models.WithPayload(tf.CreateTransaction("Tx1", 1, "Genesis")))
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 3): tf.ValidatorsSet("A", "C", "D"),
		markers.NewMarker(0, 4): tf.ValidatorsSet("A", "D"),
		markers.NewMarker(0, 5): tf.ValidatorsSet("A"),
	}))

	// ISSUE Block6
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("E").PublicKey()),
		models.WithPayload(tf.CreateTransaction("Tx2", 1, "Genesis")))
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 2): tf.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 3): tf.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.ValidatorsSet("A", "D", "E"),
		markers.NewMarker(0, 5): tf.ValidatorsSet("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A"),
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("E"),
	}))

	// ISSUE Block7
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 4): tf.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(0, 5): tf.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 6): tf.ValidatorsSet("C"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A", "C"),
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("E"),
	}))

	// ISSUE Block7.1
	tf.CreateBlock("Block7.1", models.WithStrongParents(tf.BlockIDs("Block7")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.IssueBlocks("Block7.1").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 6): tf.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 7): tf.ValidatorsSet("A"),
	}))

	// ISSUE Block7.2
	tf.CreateBlock("Block7.2", models.WithStrongParents(tf.BlockIDs("Block7.1")), models.WithLikedInsteadParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.IssueBlocks("Block7.2").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 7): tf.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 8): tf.ValidatorsSet("C"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A"),
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("E", "C"),
	}))

	// ISSUE Block8
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("C", "D", "E"),
	}))

	// ISSUE Block9
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block8")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(1, 5): tf.ValidatorsSet("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{}))

	// ISSUE Block10
	tf.CreateBlock("Block10", models.WithStrongParents(tf.BlockIDs("Block9")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block10").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 3): tf.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(1, 5): tf.ValidatorsSet("A", "B"),
		markers.NewMarker(1, 6): tf.ValidatorsSet("B"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("B", "C", "D", "E"),
	}))

	// ISSUE Block11
	tf.CreateBlock("Block11", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.IssueBlocks("Block11").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A"),
		tf.Transaction("Tx3").ID(): tf.ValidatorsSet("A"),
		tf.Transaction("Tx4").ID(): tf.ValidatorsSet(),
	}))

	// ISSUE Block12
	tf.CreateBlock("Block12", models.WithStrongParents(tf.BlockIDs("Block11")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block12").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.ValidatorsSet("A", "C", "D"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A", "D"),
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("B", "C", "E"),
		tf.Transaction("Tx4").ID(): tf.ValidatorsSet("D"),
	}))

	// ISSUE Block13
	tf.CreateBlock("Block13", models.WithStrongParents(tf.BlockIDs("Block12")), models.WithIssuer(tf.Identity("E").PublicKey()))
	tf.IssueBlocks("Block13").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.ValidatorsSet("E"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A", "D", "E"),
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("B", "C"),
		tf.Transaction("Tx4").ID(): tf.ValidatorsSet("D", "E"),
	}))

	// ISSUE Block14
	tf.CreateBlock("Block14", models.WithStrongParents(tf.BlockIDs("Block13")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block14").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.ValidatorsSet("B", "E"),
		markers.NewMarker(2, 7): tf.ValidatorsSet("B"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A", "B", "D", "E"),
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet("C"),
		tf.Transaction("Tx4").ID(): tf.ValidatorsSet("B", "D", "E"),
	}))

	// ISSUE Block15
	tf.CreateBlock("Block15", models.WithStrongParents(tf.BlockIDs("Block14")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.IssueBlocks("Block15").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.ValidatorsSet("A", "B", "E"),
		markers.NewMarker(2, 7): tf.ValidatorsSet("A", "B"),
		markers.NewMarker(2, 8): tf.ValidatorsSet("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx3").ID(): tf.ValidatorsSet(),
		tf.Transaction("Tx4").ID(): tf.ValidatorsSet("A", "B", "D", "E"),
	}))

	// ISSUE Block16
	tf.CreateBlock("Block16", models.WithStrongParents(tf.BlockIDs("Block15")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.IssueBlocks("Block16").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[identity.ID]{
		markers.NewMarker(2, 6): tf.ValidatorsSet("A", "B", "C", "E"),
		markers.NewMarker(2, 7): tf.ValidatorsSet("A", "B", "C"),
		markers.NewMarker(2, 8): tf.ValidatorsSet("A", "C"),
		markers.NewMarker(2, 9): tf.ValidatorsSet("C"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[identity.ID]{
		tf.Transaction("Tx1").ID(): tf.ValidatorsSet("A", "B", "C", "D", "E"),
		tf.Transaction("Tx2").ID(): tf.ValidatorsSet(),
		tf.Transaction("Tx4").ID(): tf.ValidatorsSet("A", "B", "C", "D", "E"),
	}))
}
