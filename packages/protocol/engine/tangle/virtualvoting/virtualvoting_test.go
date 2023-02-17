package virtualvoting

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func TestOTV_Track(t *testing.T) {
	// TODO: extend this test to cover the following cases:
	//  - when forking there is already a vote with higher power that should not be migrated
	//  - a voter that supports a marker does not support all the forked conflict's parents
	//  - test issuing votes out of order, votes have same time (possibly separate test case)

	debug.SetEnabled(true)
	defer debug.SetEnabled(false)

	workers := workerpool.NewGroup(t.Name())

	storageInstance := blockdag.NewTestStorage(t, workers)
	tf := NewTestFramework(t, workers.CreateGroup("VirtualVotingTestFramework"), New(workers.CreateGroup("VirtualVoting"),
		booker.New(workers.CreateGroup("Booker"),
			blockdag.NewTestBlockDAG(t, workers.CreateGroup("BlockDAG"), eviction.NewState(storageInstance), storageInstance.Commitments.Load),
			ledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
			booker.WithMarkerManagerOptions(
				markermanager.WithSequenceManagerOptions[models.BlockID, *booker.Block](
					markers.WithMaxPastMarkerDistance(3),
				),
			),
		),
		sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB())),
	))

	tf.CreateIdentity("A", 30)
	tf.CreateIdentity("B", 15)
	tf.CreateIdentity("C", 25)
	tf.CreateIdentity("D", 20)
	tf.CreateIdentity("E", 10)

	initialMarkerVotes := make(map[markers.Marker]*advancedset.AdvancedSet[identity.ID])
	initialConflictVotes := make(map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID])

	// ISSUE Block1
	tf.BlockDAG.CreateBlock("Block1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Genesis")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block1")

	tf.AssertBlockTracked(1)

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.Votes.ValidatorsSet("A"),
	}))

	// ISSUE Block2
	tf.BlockDAG.CreateBlock("Block2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block1")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block2")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.Votes.ValidatorsSet("A", "B"),
		markers.NewMarker(0, 2): tf.Votes.ValidatorsSet("B"),
	}))

	// ISSUE Block3
	tf.BlockDAG.CreateBlock("Block3", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block2")), models.WithIssuer(tf.Identity("C").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block3")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.Votes.ValidatorsSet("A", "B", "C"),
		markers.NewMarker(0, 2): tf.Votes.ValidatorsSet("B", "C"),
		markers.NewMarker(0, 3): tf.Votes.ValidatorsSet("C"),
	}))

	// ISSUE Block4
	tf.BlockDAG.CreateBlock("Block4", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block3")), models.WithIssuer(tf.Identity("D").PublicKey()))

	tf.BlockDAG.IssueBlocks("Block4")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.Votes.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.Votes.ValidatorsSet("B", "C", "D"),
		markers.NewMarker(0, 3): tf.Votes.ValidatorsSet("C", "D"),
		markers.NewMarker(0, 4): tf.Votes.ValidatorsSet("D"),
	}))
	// ISSUE Block5
	tf.BlockDAG.CreateBlock("Block5", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.Identity("A").PublicKey()),
		models.WithPayload(tf.Ledger.CreateTransaction("Tx1", 1, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block5")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.Votes.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 2): tf.Votes.ValidatorsSet("A", "B", "C", "D"),
		markers.NewMarker(0, 3): tf.Votes.ValidatorsSet("A", "C", "D"),
		markers.NewMarker(0, 4): tf.Votes.ValidatorsSet("A", "D"),
		markers.NewMarker(0, 5): tf.Votes.ValidatorsSet("A"),
	}))

	// ISSUE Block6
	tf.BlockDAG.CreateBlock("Block6", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block4")), models.WithIssuer(tf.Identity("E").PublicKey()),
		models.WithPayload(tf.Ledger.CreateTransaction("Tx2", 1, "Genesis")))
	tf.BlockDAG.IssueBlocks("Block6")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 1): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 2): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 3): tf.Votes.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.Votes.ValidatorsSet("A", "D", "E"),
		markers.NewMarker(0, 5): tf.Votes.ValidatorsSet("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("E"),
	}))

	// ISSUE Block7
	tf.BlockDAG.CreateBlock("Block7", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithPayload(tf.Ledger.CreateTransaction("Tx3", 1, "Tx1.0")))
	tf.BlockDAG.IssueBlocks("Block7")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 4): tf.Votes.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(0, 5): tf.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 6): tf.Votes.ValidatorsSet("C"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A", "C"),
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("E"),
	}))

	// ISSUE Block7.1
	tf.BlockDAG.CreateBlock("Block7.1", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.BlockDAG.IssueBlocks("Block7.1")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 6): tf.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 7): tf.Votes.ValidatorsSet("A"),
	}))

	// ISSUE Block7.2
	tf.BlockDAG.CreateBlock("Block7.2", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block7.1")), models.WithLikedInsteadParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*5)))
	tf.BlockDAG.IssueBlocks("Block7.2")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 7): tf.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(0, 8): tf.Votes.ValidatorsSet("C"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("E", "C"),
	}))

	// ISSUE Block8
	tf.BlockDAG.CreateBlock("Block8", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block6")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block8")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("C", "D", "E"),
	}))

	// ISSUE Block9
	tf.BlockDAG.CreateBlock("Block9", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block8")), models.WithIssuer(tf.Identity("A").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block9")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(1, 5): tf.Votes.ValidatorsSet("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{}))

	// ISSUE Block10
	tf.BlockDAG.CreateBlock("Block10", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block9")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block10")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 3): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(0, 4): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(1, 5): tf.Votes.ValidatorsSet("A", "B"),
		markers.NewMarker(1, 6): tf.Votes.ValidatorsSet("B"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("B", "C", "D", "E"),
	}))

	// ISSUE Block11
	tf.BlockDAG.CreateBlock("Block11", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block5")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithPayload(tf.Ledger.CreateTransaction("Tx4", 1, "Tx1.0")))
	tf.BlockDAG.IssueBlocks("Block11")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx3").ID(): tf.Votes.ValidatorsSet("A"),
		tf.Ledger.Transaction("Tx4").ID(): tf.Votes.ValidatorsSet(),
	}))

	// ISSUE Block12
	tf.BlockDAG.CreateBlock("Block12", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block11")), models.WithIssuer(tf.Identity("D").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block12")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.Votes.ValidatorsSet("A", "C", "D"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A", "D"),
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("B", "C", "E"),
		tf.Ledger.Transaction("Tx4").ID(): tf.Votes.ValidatorsSet("D"),
	}))

	// ISSUE Block13
	tf.BlockDAG.CreateBlock("Block13", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block12")), models.WithIssuer(tf.Identity("E").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block13")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.Votes.ValidatorsSet("A", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.Votes.ValidatorsSet("E"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A", "D", "E"),
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("B", "C"),
		tf.Ledger.Transaction("Tx4").ID(): tf.Votes.ValidatorsSet("D", "E"),
	}))

	// ISSUE Block14
	tf.BlockDAG.CreateBlock("Block14", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block13")), models.WithIssuer(tf.Identity("B").PublicKey()))
	tf.BlockDAG.IssueBlocks("Block14")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.Votes.ValidatorsSet("B", "E"),
		markers.NewMarker(2, 7): tf.Votes.ValidatorsSet("B"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A", "B", "D", "E"),
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet("C"),
		tf.Ledger.Transaction("Tx4").ID(): tf.Votes.ValidatorsSet("B", "D", "E"),
	}))

	// ISSUE Block15
	tf.BlockDAG.CreateBlock("Block15", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block14")), models.WithIssuer(tf.Identity("A").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.BlockDAG.IssueBlocks("Block15")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(0, 5): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		markers.NewMarker(2, 6): tf.Votes.ValidatorsSet("A", "B", "E"),
		markers.NewMarker(2, 7): tf.Votes.ValidatorsSet("A", "B"),
		markers.NewMarker(2, 8): tf.Votes.ValidatorsSet("A"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx3").ID(): tf.Votes.ValidatorsSet(),
		tf.Ledger.Transaction("Tx4").ID(): tf.Votes.ValidatorsSet("A", "B", "D", "E"),
	}))

	// ISSUE Block16
	tf.BlockDAG.CreateBlock("Block16", models.WithStrongParents(tf.BlockDAG.BlockIDs("Block15")), models.WithIssuer(tf.Identity("C").PublicKey()), models.WithIssuingTime(time.Now().Add(time.Minute*6)))
	tf.BlockDAG.IssueBlocks("Block16")

	tf.ValidateMarkerVoters(lo.MergeMaps(initialMarkerVotes, map[markers.Marker]*advancedset.AdvancedSet[identity.ID]{
		markers.NewMarker(2, 6): tf.Votes.ValidatorsSet("A", "B", "C", "E"),
		markers.NewMarker(2, 7): tf.Votes.ValidatorsSet("A", "B", "C"),
		markers.NewMarker(2, 8): tf.Votes.ValidatorsSet("A", "C"),
		markers.NewMarker(2, 9): tf.Votes.ValidatorsSet("C"),
	}))

	tf.ValidateConflictVoters(lo.MergeMaps(initialConflictVotes, map[utxo.TransactionID]*advancedset.AdvancedSet[identity.ID]{
		tf.Ledger.Transaction("Tx1").ID(): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
		tf.Ledger.Transaction("Tx2").ID(): tf.Votes.ValidatorsSet(),
		tf.Ledger.Transaction("Tx4").ID(): tf.Votes.ValidatorsSet("A", "B", "C", "D", "E"),
	}))
}
