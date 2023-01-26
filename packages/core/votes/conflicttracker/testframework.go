package conflicttracker

import (
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework[VotePowerType constraints.Comparable[VotePowerType]] struct {
	ConflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]

	test                         *testing.T
	optsConflictDAGTestFramework []options.Option[conflictdag.TestFramework]
	optsValidators               *sybilprotection.WeightedSet

	*VotesTestFramework
	*ConflictDAGTestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework[VotePowerType constraints.Comparable[VotePowerType]](test *testing.T, opts ...options.Option[TestFramework[VotePowerType]]) (newTestFramework *TestFramework[VotePowerType]) {
	return options.Apply(&TestFramework[VotePowerType]{
		test: test,
	}, opts, func(t *TestFramework[VotePowerType]) {
		if t.VotesTestFramework == nil {
			t.VotesTestFramework = votes.NewTestFramework(test, votes.WithValidators(
				sybilprotection.NewWeights(mapdb.NewMapDB()).NewWeightedSet(),
			))
		}

		t.ConflictDAGTestFramework = conflictdag.NewTestFramework(t.test, t.optsConflictDAGTestFramework...)

		if t.ConflictTracker == nil {
			t.ConflictTracker = NewConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType](t.ConflictDAGTestFramework.ConflictDAG, t.VotesTestFramework.Validators)
		}

		t.ConflictTracker.Events.VoterAdded.Attach(event.NewClosure(func(event *VoterEvent[utxo.TransactionID]) {
			if debug.GetEnabled() {
				t.test.Logf("CONFLICT VOTER ADDED: %v", event.ConflictID)
			}
		}))
	})
}

func (t *TestFramework[VotePowerType]) ValidateStatementResults(expectedResults map[string]*set.AdvancedSet[identity.ID]) {
	for conflictIDAlias, expectedVoters := range expectedResults {
		actualVoters := t.ConflictTracker.Voters(t.ConflictDAGTestFramework.ConflictID(conflictIDAlias))

		_ = expectedVoters.ForEach(func(expectedID identity.ID) (err error) {
			require.Truef(t.test, actualVoters.Has(expectedID), "expected voter %s to be in the set of voters of conflict %s", expectedID, conflictIDAlias)
			return nil
		})
	}
}

type VotesTestFramework = votes.TestFramework

type ConflictDAGTestFramework = conflictdag.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithVotesTestFramework[VotePowerType constraints.Comparable[VotePowerType]](votesTestFramework *votes.TestFramework) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.VotesTestFramework != nil {
			panic("VotesTestFramework already set")
		}

		tf.VotesTestFramework = votesTestFramework
	}
}

func WithConflictTracker[VotePowerType constraints.Comparable[VotePowerType]](conflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.ConflictTracker != nil {
			panic("conflict tracker already set")
		}

		tf.ConflictTracker = conflictTracker
	}
}

func WithConflictDAG[VotePowerType constraints.Comparable[VotePowerType]](conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]) options.Option[TestFramework[VotePowerType]] {
	return func(t *TestFramework[VotePowerType]) {
		if t.optsConflictDAGTestFramework == nil {
			t.optsConflictDAGTestFramework = make([]options.Option[conflictdag.TestFramework], 0)
		}

		t.optsConflictDAGTestFramework = append(t.optsConflictDAGTestFramework, conflictdag.WithConflictDAG(conflictDAG))
	}
}

func WithValidators[VotePowerType constraints.Comparable[VotePowerType]](validators *sybilprotection.WeightedSet) options.Option[TestFramework[VotePowerType]] {
	return func(t *TestFramework[VotePowerType]) {
		if t.optsConflictDAGTestFramework == nil {
			t.optsConflictDAGTestFramework = make([]options.Option[conflictdag.TestFramework], 0)
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
