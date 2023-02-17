package conflicttracker

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
)

// TestApprovalWeightManager_updateConflictVoters tests the ApprovalWeightManager's functionality regarding conflictes.
// The scenario can be found in images/approvalweight-updateConflictSupporters.png.
func TestApprovalWeightManager_updateConflictVoters(t *testing.T) {
	tf := NewDefaultFramework[votes.MockedVotePower](t)

	tf.Votes.CreateValidator("validator1", 1)
	tf.Votes.CreateValidator("validator2", 1)

	tf.ConflictDAG.CreateConflict("CS1", "Conflict1", utxo.NewTransactionIDs())
	tf.ConflictDAG.CreateConflict("CS1", "Conflict2", utxo.NewTransactionIDs())
	tf.ConflictDAG.CreateConflict("CS2", "Conflict3", utxo.NewTransactionIDs())
	tf.ConflictDAG.CreateConflict("CS2", "Conflict4", utxo.NewTransactionIDs())

	tf.ConflictDAG.CreateConflict("CS3", "Conflict1.1", tf.ConflictDAG.ConflictIDs("Conflict1"))
	tf.ConflictDAG.CreateConflict("CS3", "Conflict1.2", tf.ConflictDAG.ConflictIDs("Conflict1"))
	tf.ConflictDAG.CreateConflict("CS3", "Conflict1.3", tf.ConflictDAG.ConflictIDs("Conflict1"))

	tf.ConflictDAG.CreateConflict("CS4", "Conflict4.1", tf.ConflictDAG.ConflictIDs("Conflict4"))
	tf.ConflictDAG.CreateConflict("CS4", "Conflict4.2", tf.ConflictDAG.ConflictIDs("Conflict4"))

	tf.ConflictDAG.CreateConflict("CS5", "Conflict4.1.1", tf.ConflictDAG.ConflictIDs("Conflict4.1"))
	tf.ConflictDAG.CreateConflict("CS5", "Conflict4.1.2", tf.ConflictDAG.ConflictIDs("Conflict4.1"))

	// Issue statements in different order to make sure that no information is lost when nodes apply statements in arbitrary order

	expectedResults := map[string]*advancedset.AdvancedSet[identity.ID]{
		"Conflict1":     tf.Votes.ValidatorsSet(),
		"Conflict1.1":   tf.Votes.ValidatorsSet(),
		"Conflict1.2":   tf.Votes.ValidatorsSet(),
		"Conflict1.3":   tf.Votes.ValidatorsSet(),
		"Conflict2":     tf.Votes.ValidatorsSet(),
		"Conflict3":     tf.Votes.ValidatorsSet(),
		"Conflict4":     tf.Votes.ValidatorsSet(),
		"Conflict4.1":   tf.Votes.ValidatorsSet(),
		"Conflict4.1.1": tf.Votes.ValidatorsSet(),
		"Conflict4.1.2": tf.Votes.ValidatorsSet(),
		"Conflict4.2":   tf.Votes.ValidatorsSet(),
	}

	// statement 2: "Conflict 4.1.2", validator1
	{
		tf.Instance.TrackVote(tf.ConflictDAG.ConflictIDs("Conflict4.1.2"), tf.Votes.Validator("validator1"), votes.MockedVotePower{VotePower: 2})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*advancedset.AdvancedSet[identity.ID]{
			"Conflict4":     tf.Votes.ValidatorsSet("validator1"),
			"Conflict4.1":   tf.Votes.ValidatorsSet("validator1"),
			"Conflict4.1.2": tf.Votes.ValidatorsSet("validator1"),
		}))
	}

	// statement 1: "Conflict 1.1 + Conflict 4.1.1", validator1
	{
		tf.Instance.TrackVote(tf.ConflictDAG.ConflictIDs("Conflict1.1", "Conflict4.1.1"), tf.Votes.Validator("validator1"), votes.MockedVotePower{VotePower: 1})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*advancedset.AdvancedSet[identity.ID]{
			"Conflict1":   tf.Votes.ValidatorsSet("validator1"),
			"Conflict1.1": tf.Votes.ValidatorsSet("validator1"),
		}))
	}

	// statement 3: "Conflict 2", validator1
	{
		tf.Instance.TrackVote(tf.ConflictDAG.ConflictIDs("Conflict2"), tf.Votes.Validator("validator1"), votes.MockedVotePower{VotePower: 3})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*advancedset.AdvancedSet[identity.ID]{
			"Conflict1":   tf.Votes.ValidatorsSet(),
			"Conflict1.1": tf.Votes.ValidatorsSet(),
			"Conflict2":   tf.Votes.ValidatorsSet("validator1"),
		}))
	}

	// statement 4: "Conflict1.2 + Conflict4.1.2", validator2
	{
		tf.Instance.TrackVote(tf.ConflictDAG.ConflictIDs("Conflict1.2", "Conflict4.1.2"), tf.Votes.Validator("validator2"), votes.MockedVotePower{VotePower: 3})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*advancedset.AdvancedSet[identity.ID]{
			"Conflict1":     tf.Votes.ValidatorsSet("validator2"),
			"Conflict1.2":   tf.Votes.ValidatorsSet("validator2"),
			"Conflict4.1.2": tf.Votes.ValidatorsSet("validator1", "validator2"),
			"Conflict4.1":   tf.Votes.ValidatorsSet("validator1", "validator2"),
			"Conflict4":     tf.Votes.ValidatorsSet("validator1", "validator2"),
		}))
	}

	// statement 5: "Conflict 3", validator2
	{
		tf.Instance.TrackVote(tf.ConflictDAG.ConflictIDs("Conflict3"), tf.Votes.Validator("validator2"), votes.MockedVotePower{VotePower: 5})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*advancedset.AdvancedSet[identity.ID]{
			"Conflict3":     tf.Votes.ValidatorsSet("validator2"),
			"Conflict4.1.2": tf.Votes.ValidatorsSet("validator1"),
			"Conflict4.1":   tf.Votes.ValidatorsSet("validator1"),
			"Conflict4":     tf.Votes.ValidatorsSet("validator1"),
		}))
	}
}
