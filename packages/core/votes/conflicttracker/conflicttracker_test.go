package conflicttracker

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/utxo"
)

// TestApprovalWeightManager_updateConflictVoters tests the ApprovalWeightManager's functionality regarding conflictes.
// The scenario can be found in images/approvalweight-updateConflictSupporters.png.
func TestApprovalWeightManager_updateConflictVoters(t *testing.T) {
	tf := NewTestFramework[votes.MockedVotePower](t)

	tf.CreateValidator("validator1")
	tf.CreateValidator("validator2")

	tf.CreateConflict("CS1", "Conflict1", utxo.NewTransactionIDs())
	tf.CreateConflict("CS1", "Conflict2", utxo.NewTransactionIDs())
	tf.CreateConflict("CS2", "Conflict3", utxo.NewTransactionIDs())
	tf.CreateConflict("CS2", "Conflict4", utxo.NewTransactionIDs())

	tf.CreateConflict("CS3", "Conflict1.1", tf.ConflictIDs("Conflict1"))
	tf.CreateConflict("CS3", "Conflict1.2", tf.ConflictIDs("Conflict1"))
	tf.CreateConflict("CS3", "Conflict1.3", tf.ConflictIDs("Conflict1"))

	tf.CreateConflict("CS4", "Conflict4.1", tf.ConflictIDs("Conflict4"))
	tf.CreateConflict("CS4", "Conflict4.2", tf.ConflictIDs("Conflict4"))

	tf.CreateConflict("CS5", "Conflict4.1.1", tf.ConflictIDs("Conflict4.1"))
	tf.CreateConflict("CS5", "Conflict4.1.2", tf.ConflictIDs("Conflict4.1"))

	// Issue statements in different order to make sure that no information is lost when nodes apply statements in arbitrary order

	expectedResults := map[string]*set.AdvancedSet[*validator.Validator]{
		"Conflict1":     tf.Validators(),
		"Conflict1.1":   tf.Validators(),
		"Conflict1.2":   tf.Validators(),
		"Conflict1.3":   tf.Validators(),
		"Conflict2":     tf.Validators(),
		"Conflict3":     tf.Validators(),
		"Conflict4":     tf.Validators(),
		"Conflict4.1":   tf.Validators(),
		"Conflict4.1.1": tf.Validators(),
		"Conflict4.1.2": tf.Validators(),
		"Conflict4.2":   tf.Validators(),
	}

	// statement 2: "Conflict 4.1.2", validator1
	{
		tf.ConflictTracker.TrackVote(tf.ConflictIDs("Conflict4.1.2"), tf.Validator("validator1").ID(), votes.MockedVotePower{2})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*set.AdvancedSet[*validator.Validator]{
			"Conflict4":     tf.Validators("validator1"),
			"Conflict4.1":   tf.Validators("validator1"),
			"Conflict4.1.2": tf.Validators("validator1"),
		}))
	}

	// statement 1: "Conflict 1.1 + Conflict 4.1.1", validator1
	{
		tf.ConflictTracker.TrackVote(tf.ConflictIDs("Conflict1.1", "Conflict4.1.1"), tf.Validator("validator1").ID(), votes.MockedVotePower{1})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*set.AdvancedSet[*validator.Validator]{
			"Conflict1":   tf.Validators("validator1"),
			"Conflict1.1": tf.Validators("validator1"),
		}))
	}

	// statement 3: "Conflict 2", validator1
	{
		tf.ConflictTracker.TrackVote(tf.ConflictIDs("Conflict2"), tf.Validator("validator1").ID(), votes.MockedVotePower{3})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*set.AdvancedSet[*validator.Validator]{
			"Conflict1":   tf.Validators(),
			"Conflict1.1": tf.Validators(),
			"Conflict2":   tf.Validators("validator1"),
		}))
	}

	// statement 4: "Conflict1.2 + Conflict4.1.2", validator2
	{
		tf.ConflictTracker.TrackVote(tf.ConflictIDs("Conflict1.2", "Conflict4.1.2"), tf.Validator("validator2").ID(), votes.MockedVotePower{3})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*set.AdvancedSet[*validator.Validator]{
			"Conflict1":     tf.Validators("validator2"),
			"Conflict1.2":   tf.Validators("validator2"),
			"Conflict4.1.2": tf.Validators("validator1", "validator2"),
			"Conflict4.1":   tf.Validators("validator1", "validator2"),
			"Conflict4":     tf.Validators("validator1", "validator2"),
		}))
	}

	// statement 5: "Conflict 3", validator2
	{
		tf.ConflictTracker.TrackVote(tf.ConflictIDs("Conflict3"), tf.Validator("validator2").ID(), votes.MockedVotePower{5})

		tf.ValidateStatementResults(lo.MergeMaps(expectedResults, map[string]*set.AdvancedSet[*validator.Validator]{
			"Conflict3":     tf.Validators("validator2"),
			"Conflict4.1.2": tf.Validators("validator1"),
			"Conflict4.1":   tf.Validators("validator1"),
			"Conflict4":     tf.Validators("validator1"),
		}))
	}
}
