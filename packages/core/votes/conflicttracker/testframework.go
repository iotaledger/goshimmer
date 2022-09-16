package conflicttracker

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/utxo"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework[VotePowerType votes.VotePower[VotePowerType]] struct {
	ConflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]

	test                         *testing.T
	optsConflictDAGTestFramework []options.Option[conflictdag.TestFramework]

	*VotesTestFramework
	*ConflictDAGTestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework[VotePowerType votes.VotePower[VotePowerType]](test *testing.T, opts ...options.Option[TestFramework[VotePowerType]]) (newTestFramework *TestFramework[VotePowerType]) {
	return options.Apply(&TestFramework[VotePowerType]{
		test: test,
	}, opts, func(t *TestFramework[VotePowerType]) {
		if t.VotesTestFramework == nil {
			t.VotesTestFramework = votes.NewTestFramework(test)
		}

		t.ConflictDAGTestFramework = conflictdag.NewTestFramework(t.test, t.optsConflictDAGTestFramework...)

		if t.ConflictTracker == nil {
			t.ConflictTracker = NewConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType](t.ConflictDAG(), t.ValidatorSet)
		}
	})
}

func (t *TestFramework[VotePowerType]) ValidateStatementResults(expectedResults map[string]*set.AdvancedSet[*validator.Validator]) {
	for conflictIDAlias, expectedVoters := range expectedResults {
		actualVoters := t.ConflictTracker.Voters(t.ConflictID(conflictIDAlias))

		assert.Truef(t.test, expectedVoters.Equal(votes.ValidatorSetToAdvancedSet(actualVoters)), "%s expected to have %d voters but got %d", conflictIDAlias, expectedVoters.Size(), actualVoters.Size())
	}
}

type VotesTestFramework = votes.TestFramework

type ConflictDAGTestFramework = conflictdag.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithVotesTestFramework[VotePowerType votes.VotePower[VotePowerType]](votesTestFramework *votes.TestFramework) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.VotesTestFramework != nil {
			panic("VotesTestFramework already set")
		}

		tf.VotesTestFramework = votesTestFramework
	}
}

func WithConflictTracker[VotePowerType votes.VotePower[VotePowerType]](conflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.ConflictTracker != nil {
			panic("conflict tracker already set")
		}

		tf.ConflictTracker = conflictTracker
	}
}

func WithConflictDAG[VotePowerType votes.VotePower[VotePowerType]](conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]) options.Option[TestFramework[VotePowerType]] {
	return func(t *TestFramework[VotePowerType]) {
		if t.optsConflictDAGTestFramework == nil {
			t.optsConflictDAGTestFramework = make([]options.Option[conflictdag.TestFramework], 0)
		}

		t.optsConflictDAGTestFramework = append(t.optsConflictDAGTestFramework, conflictdag.WithConflictDAG(conflictDAG))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
