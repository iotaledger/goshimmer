package otv

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

func TestOTV_Track(t *testing.T) {
	debug.SetEnabled(true)

	tf := NewTestFramework(t)
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
	tf.CreateBlock("Block7", models.WithStrongParents(tf.BlockIDs("Block5")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithPayload(tf.CreateTransaction("Tx3", 1, "Tx2.0")))

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

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{
		markers.NewMarker(0, 6): tf.Validators("A", "C"),
		markers.NewMarker(0, 7): tf.Validators("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{}))

	// ISSUE Block8
	tf.CreateBlock("Block8", models.WithStrongParents(tf.BlockIDs("Block6")), models.WithIssuer(tf.Identity("D").PublicKey()))

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*set.AdvancedSet[*validator.Validator]{}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*set.AdvancedSet[*validator.Validator]{
		tf.Transaction("Tx2").ID(): tf.Validators("D", "E"),
	}))
	// TODO: this tests reorg, which is not yet supported
	// ISSUE Block9
	// 		testFramework.CreateBlock("Block9", WithStrongParents("Block8"), WithIssuer(nodes["A"].PublicKey()))
	//
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.30)
	//
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.25)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.60)
	//
	// 		IssueAndValidateBlockApproval(t, "Block9", testEventMock, testFramework, map[string]float64{
	// 			"Conflict1": 0.25,
	// 			"Conflict2": 0.60,
	// 		}, map[markers.Marker]float64{
	// 			markers.NewMarker(0, 1): 1,
	// 			markers.NewMarker(0, 2): 1,
	// 			markers.NewMarker(0, 3): 0.85,
	// 			markers.NewMarker(0, 4): 0.85,
	// 			markers.NewMarker(0, 5): 0.55,
	// 			markers.NewMarker(0, 6): 0.55,
	// 			markers.NewMarker(0, 7): 0.30,
	// 			markers.NewMarker(1, 5): 0.30,
	// 		})
	// 	// ISSUE Block10
	// 		testFramework.CreateBlock("Block10", WithStrongParents("Block9"), WithIssuer(nodes["B"].PublicKey()))
	//
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 3), 1.0)
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 4), 1.0)
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 5), 0.45)
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(1, 6), 0.15)
	//
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.75)
	//
	// 		IssueAndValidateBlockApproval(t, "Block10", testEventMock, testFramework, map[string]float64{
	// 			"Conflict1": 0.25,
	// 			"Conflict2": 0.75,
	// 		}, map[markers.Marker]float64{
	// 			markers.NewMarker(0, 1): 1,
	// 			markers.NewMarker(0, 2): 1,
	// 			markers.NewMarker(0, 3): 1,
	// 			markers.NewMarker(0, 4): 1,
	// 			markers.NewMarker(0, 5): 0.55,
	// 			markers.NewMarker(0, 6): 0.55,
	// 			markers.NewMarker(0, 7): 0.30,
	// 			markers.NewMarker(1, 5): 0.45,
	// 			markers.NewMarker(1, 6): 0.15,
	// 		})
	// 	// ISSUE Block11
	// 		testFramework.CreateBlock("Block11", WithStrongParents("Block5"), WithIssuer(nodes["A"].PublicKey()), WithInputs("B"), WithOutput("D", 500))
	// 		testFramework.RegisterConflictID("Conflict3", "Block7")
	// 		testFramework.RegisterConflictID("Conflict4", "Block11")
	//
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.55)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.45)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict3"), 0.25)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.30)
	//
	// 		IssueAndValidateBlockApproval(t, "Block11", testEventMock, testFramework, map[string]float64{
	// 			"Conflict1": 0.55,
	// 			"Conflict2": 0.45,
	// 			"Conflict3": 0.25,
	// 			"Conflict4": 0.30,
	// 		}, map[markers.Marker]float64{
	// 			markers.NewMarker(0, 1): 1,
	// 			markers.NewMarker(0, 2): 1,
	// 			markers.NewMarker(0, 3): 1,
	// 			markers.NewMarker(0, 4): 1,
	// 			markers.NewMarker(0, 5): 0.55,
	// 			markers.NewMarker(0, 6): 0.55,
	// 			markers.NewMarker(0, 7): 0.30,
	// 			markers.NewMarker(1, 5): 0.45,
	// 			markers.NewMarker(1, 6): 0.15,
	// 		})
	// 	// ISSUE Block12
	// 		testFramework.CreateBlock("Block12", WithStrongParents("Block11"), WithIssuer(nodes["D"].PublicKey()))
	//
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.75)
	//
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.75)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.25)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.50)
	//
	// 		IssueAndValidateBlockApproval(t, "Block12", testEventMock, testFramework, map[string]float64{
	// 			"Conflict1": 0.75,
	// 			"Conflict2": 0.25,
	// 			"Conflict3": 0.25,
	// 			"Conflict4": 0.50,
	// 		}, map[markers.Marker]float64{
	// 			markers.NewMarker(0, 1): 1,
	// 			markers.NewMarker(0, 2): 1,
	// 			markers.NewMarker(0, 3): 1,
	// 			markers.NewMarker(0, 4): 1,
	// 			markers.NewMarker(0, 5): 0.75,
	// 			markers.NewMarker(0, 6): 0.55,
	// 			markers.NewMarker(0, 7): 0.30,
	// 			markers.NewMarker(1, 5): 0.45,
	// 			markers.NewMarker(1, 6): 0.15,
	// 		})
	// 	// ISSUE Block13
	// 		testFramework.CreateBlock("Block13", WithStrongParents("Block12"), WithIssuer(nodes["E"].PublicKey()))
	//
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 0.85)
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 6), 0.10)
	//
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 0.85)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.15)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.60)
	//
	// 		IssueAndValidateBlockApproval(t, "Block13", testEventMock, testFramework, map[string]float64{
	// 			"Conflict1": 0.85,
	// 			"Conflict2": 0.15,
	// 			"Conflict3": 0.25,
	// 			"Conflict4": 0.60,
	// 		}, map[markers.Marker]float64{
	// 			markers.NewMarker(0, 1): 1,
	// 			markers.NewMarker(0, 2): 1,
	// 			markers.NewMarker(0, 3): 1,
	// 			markers.NewMarker(0, 4): 1,
	// 			markers.NewMarker(0, 5): 0.85,
	// 			markers.NewMarker(0, 6): 0.55,
	// 			markers.NewMarker(0, 7): 0.30,
	// 			markers.NewMarker(1, 5): 0.45,
	// 			markers.NewMarker(1, 6): 0.15,
	// 			markers.NewMarker(2, 6): 0.10,
	// 		})
	// 	// ISSUE Block14
	// 		testFramework.CreateBlock("Block14", WithStrongParents("Block13"), WithIssuer(nodes["B"].PublicKey()))
	//
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(0, 5), 1.00)
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 6), 0.25)
	// 		testEventMock.Expect("MarkerWeightChanged", markers.NewMarker(2, 7), 0.15)
	//
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict1"), 1.0)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict2"), 0.0)
	// 		testEventMock.Expect("ConflictWeightChanged", testFramework.ConflictID("Conflict4"), 0.75)
	//
	// 		IssueAndValidateBlockApproval(t, "Block14", testEventMock, testFramework, map[string]float64{
	// 			"Conflict1": 1,
	// 			"Conflict2": 0,
	// 			"Conflict3": 0.25,
	// 			"Conflict4": 0.75,
	// 		}, map[markers.Marker]float64{
	// 			markers.NewMarker(0, 1): 1,
	// 			markers.NewMarker(0, 2): 1,
	// 			markers.NewMarker(0, 3): 1,
	// 			markers.NewMarker(0, 4): 1,
	// 			markers.NewMarker(0, 5): 1,
	// 			markers.NewMarker(0, 6): 0.55,
	// 			markers.NewMarker(0, 7): 0.30,
	// 			markers.NewMarker(1, 5): 0.45,
	// 			markers.NewMarker(1, 6): 0.15,
	// 			markers.NewMarker(2, 6): 0.25,
	// 			markers.NewMarker(2, 7): 0.15,
	// 		})
}
