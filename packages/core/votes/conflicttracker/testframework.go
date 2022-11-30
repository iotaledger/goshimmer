package conflicttracker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage/permanent"
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
				sybilprotection.NewWeights(mapdb.NewMapDB(), permanent.NewSettings(test.TempDir()+"/settings")).WeightedSet(),
			))
		}

		t.ConflictDAGTestFramework = conflictdag.NewTestFramework(t.test, t.optsConflictDAGTestFramework...)

		if t.ConflictTracker == nil {
			t.ConflictTracker = NewConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType](t.ConflictDAG(), t.VotesTestFramework.Validators)
		}
	})
}

func (t *TestFramework[VotePowerType]) ValidateStatementResults(expectedResults map[string]*set.AdvancedSet[identity.ID]) {
	for conflictIDAlias, expectedVoters := range expectedResults {
		actualVoters := t.ConflictTracker.Voters(t.ConflictID(conflictIDAlias))

		expectedVoters.ForEach(func(expectedID identity.ID) (err error) {
			var found bool
			actualVoters.ForEachWeighted(func(actualID identity.ID, actualWeight int64) error {
				if actualID == expectedID {
					found = true
					validatorWeight, exists := t.Validators.Weights.Weight(actualID)
					if !exists {
						validatorWeight = sybilprotection.NewWeight(0, -1)
					}
					expectedWeight := validatorWeight.Value
					assert.Equalf(t.test, expectedWeight, actualWeight, "validator %s weight does not match: expected %d actual %d", expectedID, expectedWeight, actualWeight)
				}
				return nil
			})

			if !found {
				t.test.Fatalf("validators do not match: expected %s actual %s", expectedVoters, actualVoters.Members())
			}

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
