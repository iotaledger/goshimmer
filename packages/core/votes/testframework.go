package votes

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type conflictDAGTestFramework = *conflictdag.TestFramework
type markersTestFramework = *markers.TestFramework

type TestFramework[VotePowerType VotePower[VotePowerType]] struct {
	conflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]
	sequenceTracker *SequenceTracker[VotePowerType]
	ValidatorSet    *validator.Set

	validatorsByAlias map[string]*validator.Validator

	conflictDAG     *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]
	sequenceManager *markers.SequenceManager

	test *testing.T

	conflictDAGTestFramework
	markersTestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework[VotePowerType VotePower[VotePowerType]](test *testing.T, opts ...options.Option[TestFramework[VotePowerType]]) (newTestFramework *TestFramework[VotePowerType]) {
	return options.Apply(&TestFramework[VotePowerType]{
		validatorsByAlias: make(map[string]*validator.Validator),
		test:              test,
	}, opts, func(t *TestFramework[VotePowerType]) {
		if t.ValidatorSet == nil {
			t.ValidatorSet = validator.NewSet()
		}

		if t.conflictDAG == nil {
			t.conflictDAGTestFramework = conflictdag.NewTestFramework(t.test, conflictdag.WithConflictDAG(t.conflictDAG))
		}
		if t.sequenceManager == nil {
			t.markersTestFramework = markers.NewTestFramework(t.test, markers.WithSequenceManager(t.sequenceManager))
		}

		t.SequenceTracker().Events.VoterAdded.Hook(event.NewClosure(func(evt *SequenceVoterEvent) {
			if debug.GetEnabled() {
				t.test.Logf("VOTER ADDED: %v", evt.Marker)
			}
		}))
	})
}

func (t *TestFramework[VotePowerType]) CreateValidator(alias string, opts ...options.Option[validator.Validator]) *validator.Validator {
	return t.CreateValidatorWithID(alias, lo.PanicOnErr(identity.RandomIDInsecure()), opts...)
}

func (t *TestFramework[VotePowerType]) CreateValidatorWithID(alias string, id identity.ID, opts ...options.Option[validator.Validator]) *validator.Validator {
	voter := validator.New(id, opts...)

	t.validatorsByAlias[alias] = voter
	t.ValidatorSet.Add(voter)

	return voter
}

func (t *TestFramework[VotePowerType]) Validator(alias string) (v *validator.Validator) {
	v, ok := t.validatorsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Validator alias %s not registered", alias))
	}

	return
}

func (t *TestFramework[VotePowerType]) Validators(aliases ...string) (validators *set.AdvancedSet[*validator.Validator]) {
	validators = set.NewAdvancedSet[*validator.Validator]()
	for _, alias := range aliases {
		validators.Add(t.Validator(alias))
	}

	return
}

func (t *TestFramework[VotePowerType]) ValidateStatementResults(expectedResults map[string]*set.AdvancedSet[*validator.Validator]) {
	for conflictIDAlias, expectedVoters := range expectedResults {
		actualVoters := t.ConflictTracker().Voters(t.ConflictID(conflictIDAlias))

		assert.Truef(t.test, actualVoters.Equal(expectedVoters), "%s expected to have %d voters but got %d", conflictIDAlias, expectedVoters.Size(), actualVoters.Size())
	}
}

func (t *TestFramework[VotePowerType]) ValidateStructureDetailsVoters(expectedVoters map[string]*set.AdvancedSet[*validator.Validator]) {
	for markerAlias, expectedVotersOfMarker := range expectedVoters {
		// sanity check
		assert.Equal(t.test, markerAlias, fmt.Sprintf("%d,%d", t.StructureDetails(markerAlias).PastMarkers().Marker().SequenceID(), t.StructureDetails(markerAlias).PastMarkers().Marker().Index()))

		voters := t.SequenceTracker().Voters(t.StructureDetails(markerAlias).PastMarkers().Marker())

		assert.True(t.test, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", markerAlias, expectedVotersOfMarker.Size(), voters.Size())
	}
}

type mockVotePower struct {
	votePower int
}

func (p mockVotePower) CompareTo(other mockVotePower) int {
	if p.votePower-other.votePower < 0 {
		return -1
	} else if p.votePower-other.votePower > 0 {
		return 1
	} else {
		return 0
	}
}

func (t *TestFramework[VotePowerType]) SequenceTracker() (sequenceTracker *SequenceTracker[VotePowerType]) {
	if t.sequenceTracker == nil {
		t.sequenceTracker = NewSequenceTracker[VotePowerType](t.ValidatorSet, t.SequenceManager().Sequence, func(sequenceID markers.SequenceID) markers.Index { return 0 })
	}

	return t.sequenceTracker
}

func (t *TestFramework[VotePowerType]) ConflictTracker() (conflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]) {
	if t.conflictTracker == nil {
		t.conflictTracker = NewConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType](t.ConflictDAG(), t.ValidatorSet)
	}

	return t.conflictTracker
}

func WithValidatorSet[VotePowerType VotePower[VotePowerType]](validatorSet *validator.Set) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.ValidatorSet != nil {
			panic("validator set already set")
		}
		tf.ValidatorSet = validatorSet
	}
}

func WithSequenceTracker[VotePowerType VotePower[VotePowerType]](sequenceTracker *SequenceTracker[VotePowerType]) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.sequenceTracker != nil {
			panic("sequence tracker already set")
		}
		tf.sequenceTracker = sequenceTracker
	}
}

func WithConflictTracker[VotePowerType VotePower[VotePowerType]](conflictTracker *ConflictTracker[utxo.TransactionID, utxo.OutputID, VotePowerType]) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.conflictTracker != nil {
			panic("conflict tracker already set")
		}
		tf.conflictTracker = conflictTracker
	}
}

func WithConflictDAG[VotePowerType VotePower[VotePowerType]](conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.conflictDAG != nil {
			panic("conflict DAG already set")
		}
		tf.conflictDAG = conflictDAG
	}
}

func WithSequenceManager[VotePowerType VotePower[VotePowerType]](sequenceManager *markers.SequenceManager) options.Option[TestFramework[VotePowerType]] {
	return func(tf *TestFramework[VotePowerType]) {
		if tf.sequenceManager != nil {
			panic("sequence manager already set")
		}
		tf.sequenceManager = sequenceManager
	}
}
