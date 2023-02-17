package conflicttracker

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework[VotePowerType constraints.Comparable[VotePowerType]] struct {
	test        *testing.T
	Instance    *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]
	Votes       *votes.TestFramework
	ConflictDAG *conflictdag.TestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework[VotePowerType constraints.Comparable[VotePowerType]](test *testing.T, votesTF *votes.TestFramework, conflictDAGTF *conflictdag.TestFramework, conflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]) *TestFramework[VotePowerType] {
	t := &TestFramework[VotePowerType]{
		test:        test,
		Instance:    conflictTracker,
		Votes:       votesTF,
		ConflictDAG: conflictDAGTF,
	}

	event.Hook(t.Instance.Events.VoterAdded, func(event *VoterEvent[utxo.TransactionID]) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT VOTER ADDED: %v, %v, %v", event.ConflictID, event.Voter, event.Opinion)
		}
	})

	return t
}

func NewDefaultFramework[VotePowerType constraints.Comparable[VotePowerType]](t *testing.T) *TestFramework[VotePowerType] {
	votesTF := votes.NewTestFramework(t, sybilprotection.NewWeights(mapdb.NewMapDB()).NewWeightedSet())
	conflictDAGTF := conflictdag.NewTestFramework(t, conflictdag.New[utxo.TransactionID, utxo.OutputID]())
	return NewTestFramework[VotePowerType](t,
		votesTF,
		conflictDAGTF,
		NewConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType](conflictDAGTF.Instance, votesTF.Validators),
	)
}

func (t *TestFramework[VotePowerType]) ValidateStatementResults(expectedResults map[string]*set.AdvancedSet[identity.ID]) {
	for conflictIDAlias, expectedVoters := range expectedResults {
		actualVoters := t.Instance.Voters(t.ConflictDAG.ConflictID(conflictIDAlias))

		_ = expectedVoters.ForEach(func(expectedID identity.ID) (err error) {
			require.Truef(t.test, actualVoters.Has(expectedID), "expected voter %s to be in the set of voters of conflict %s", expectedID, conflictIDAlias)
			return nil
		})
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
