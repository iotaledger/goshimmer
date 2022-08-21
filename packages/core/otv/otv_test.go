package otv

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/booker"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

func TestOTV_Track(t *testing.T) {
	// TODO: extend this test to cover the following cases:
	//  - when forking there is already a vote with higher power that should not be migrated
	//  - a voter that supports a marker does not support all the forked conflict's parents
	//  - test issuing votes out of order, votes have same time (possibly separate test case)

	debug.SetEnabled(true)

	tf := NewTestFramework(t, WithOnTangleVotingOptions(WithBookerOptions(booker.WithMarkerManagerOptions(booker.WithSequenceManagerOptions(markers.WithMaxPastMarkerDistance(3))))))
	tf.CreateIdentity("A", validator.WithWeight(30))
	tf.CreateIdentity("B", validator.WithWeight(15))
	tf.CreateIdentity("C", validator.WithWeight(25))
	tf.CreateIdentity("D", validator.WithWeight(20))
	tf.CreateIdentity("E", validator.WithWeight(10))

	initialMarkerVotes := make(map[markers.Marker]*set.AdvancedSet[*validator.Validator])
	initialConflictVotes := make(map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator])

	// ISSUE Block1
	tf.CreateBlock("Block1", models.WithStrongParents(tf.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

	tf.AssertBlockTracked(1)

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 1): tf.Validators("A"),
	}))

	// ISSUE Block2
	tf.CreateBlock("Block2", models.WithStrongParents(tf.BlockIDs("Block1")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 1): tf.Validators("A", "B"),
		markers.NewMarker(0, 2): tf.Validators("B"),
	}))

	// ISSUE Block3
	tf.CreateBlock("Block3", models.WithStrongParents(tf.BlockIDs("Block2")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 1): tf.Validators("A", "B", "C"),
		markers.NewMarker(0, 2): tf.Validators("B", "C"),
		markers.NewMarker(0, 3): tf.Validators("C"),
	}))

	// ISSUE Block4
	tf.CreateBlock("Block4", models.WithStrongParents(tf.BlockIDs("Block3")), models.WithIssuer(tf.Identity("D").PublicKey()))

	tf.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 1): tf.Validators("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.Validators("B", "C", "D"),
		markers.NewMarker(0, 3): tf.Validators("C", "D"),
		markers.NewMarker(0, 4): tf.Validators("D"),
	}))
	// ISSUE Block5
	tf.CreateBlock("Block5", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()),
		models.WithPayload(tf.CreateTransaction("Tx1", 1, "Genesis")))
	tf.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 1): tf.Validators("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.Validators("A", "B", "C", "D"),
		markers.NewMarker(0, 3): tf.Validators("A", "C", "D"),
		markers.NewMarker(0, 4): tf.Validators("A", "D"),
		markers.NewMarker(0, 5): tf.Validators("A"),
	}))

	// ISSUE Block6
	tf.CreateBlock("Block6", models.WithStrongParents(tf.BlockIDs("Block4")), models.WithIssuer(tf.Identity("E").PublicKey()),
		models.WithPayload(tf.CreateTransaction("Tx2", 1, "Genesis")))
	tf.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 1): tf.Validators("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 2): tf.Validators("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 3): tf.Validators("A", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.Validators("A", "D", "E"),
		markers.NewMarker(0, 5): tf.Validators("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx1").ID(): tf.Validators("A"),
		tf.Transaction("Tx2").ID(): tf.Validators("E"),
	}))

	// ISSUE Block7
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 4): tf.Validators("A", "C", "D", "E"),
		markers.NewMarker(0, 5): tf.Validators("A", "C"),
		markers.NewMarker(0, 6): tf.Validators("C"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx1").ID(): tf.Validators("A", "C"),
		tf.Transaction("Tx2").ID(): tf.Validators("E"),
	}))

	// ISSUE Block7.1
	tf.CreateBlock("Block7.1", models.WithStrongParents(tf.BlockIDs("Block7")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block7.1").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 6): tf.Validators("A", "C"),
		markers.NewMarker(0, 7): tf.Validators("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{}))

	// ISSUE Block8
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx2").ID(): tf.Validators("D", "E"),
	}))

	// ISSUE Block9
	tf.CreateBlock("Block9", models.WithStrongParents(tf.BlockIDs("Block8")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(1, 5): tf.Validators("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx1").ID(): tf.Validators("C"),
		tf.Transaction("Tx2").ID(): tf.Validators("A", "D", "E"),
	}))

	// ISSUE Block10
	tf.CreateBlock("Block10", models.WithStrongParents(tf.BlockIDs("Block9")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block10").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 3): tf.Validators("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.Validators("A", "B", "C", "D", "E"),
		markers.NewMarker(1, 5): tf.Validators("A", "B"),
		markers.NewMarker(1, 6): tf.Validators("B"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx2").ID(): tf.Validators("A", "B", "D", "E"),
	}))

	// ISSUE Block11
	tf.CreateBlock("Block11", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.IssueBlocks("Block11").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx1").ID(): tf.Validators("A", "C"),
		tf.Transaction("Tx2").ID(): tf.Validators("B", "D", "E"),

		tf.Transaction("Tx3").ID(): tf.Validators("C"),
		tf.Transaction("Tx4").ID(): tf.Validators("A"),
	}))

	// ISSUE Block12
	tf.CreateBlock("Block12", models.WithStrongParents(tf.BlockIDs("Block11")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.IssueBlocks("Block12").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 5): tf.Validators("A", "C", "D"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx1").ID(): tf.Validators("A", "C", "D"),
		tf.Transaction("Tx2").ID(): tf.Validators("B", "E"),
		tf.Transaction("Tx4").ID(): tf.Validators("A", "D"),
	}))

	// ISSUE Block13
	tf.CreateBlock("Block13", models.WithStrongParents(tf.BlockIDs("Block12")), models.WithIssuer(tf.Identity("E").PublicKey()))
	tf.IssueBlocks("Block13").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 5): tf.Validators("A", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.Validators("E"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx1").ID(): tf.Validators("A", "C", "D", "E"),
		tf.Transaction("Tx2").ID(): tf.Validators("B"),
		tf.Transaction("Tx4").ID(): tf.Validators("A", "D", "E"),
	}))

	// ISSUE Block14
	tf.CreateBlock("Block14", models.WithStrongParents(tf.BlockIDs("Block13")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.IssueBlocks("Block14").WaitUntilAllTasksProcessed()

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 5): tf.Validators("A", "B", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.Validators("B", "E"),
		markers.NewMarker(2, 7): tf.Validators("B"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx1").ID(): tf.Validators("A", "B", "C", "D", "E"),
		tf.Transaction("Tx2").ID(): tf.Validators(),
		tf.Transaction("Tx4").ID(): tf.Validators("A", "B", "D", "E"),
	}))
}
