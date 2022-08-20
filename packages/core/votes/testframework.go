package votes

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type conflictDAGTestFramework = *conflictdag.TestFramework
type markersTestFramework = *markers.TestFramework

type TestFramework struct {
	VotesTracker    *ConflictTracker[utxo.TransactionID, utxo.OutputID, mockVotePower]
	SequenceTracker *SequenceTracker
	validatorSet    *validator.Set

	validatorsByAlias map[string]*validator.Validator

	t *testing.T

	conflictDAGTestFramework
	markersTestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(t *testing.T) (newFramework *TestFramework) {
	conflictDAGTf := conflictdag.NewTestFramework(t)
	markersTf := markers.NewTestFramework(t)

	validatorSet := validator.NewSet()
	return &TestFramework{
		VotesTracker:             NewConflictTracker[utxo.TransactionID, utxo.OutputID, mockVotePower](conflictDAGTf.ConflictDAG, validatorSet),
		SequenceTracker:          NewSequenceTracker(markersTf.SequenceManager, validatorSet, func(sequenceID markers.SequenceID) markers.Index { return 0 }),
		conflictDAGTestFramework: conflictDAGTf,
		markersTestFramework:     markersTf,
		validatorSet:             validatorSet,
		validatorsByAlias:        make(map[string]*validator.Validator),
		t:                        t,
	}
}

func (t *TestFramework) CreateValidator(alias string, opts ...options.Option[validator.Validator]) *validator.Validator {
	voter := validator.New(lo.PanicOnErr(identity.RandomID()), opts...)

	t.validatorsByAlias[alias] = voter
	t.validatorSet.Add(voter)

	return voter
}

func (t *TestFramework) Validator(alias string) (v *validator.Validator) {
	v, ok := t.validatorsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Validator alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) Validators(aliases ...string) (validators *set.AdvancedSet[*validator.Validator]) {
	validators = set.NewAdvancedSet[*validator.Validator]()
	for _, alias := range aliases {
		validators.Add(t.Validator(alias))
	}

	return
}

func (t *TestFramework) validateStatementResults(expectedResults map[string]*set.AdvancedSet[*validator.Validator]) {
	for conflictIDAlias, expectedVoters := range expectedResults {
		actualVoters := t.VotesTracker.Voters(t.ConflictID(conflictIDAlias))

		assert.Truef(t.t, actualVoters.Equal(expectedVoters), "%s expected to have %d voters but got %d", conflictIDAlias, expectedVoters.Size(), actualVoters.Size())
	}
}

func (t *TestFramework) validateMarkerVoters(expectedVoters map[string]*set.AdvancedSet[*validator.Validator]) {
	for markerAlias, expectedVotersOfMarker := range expectedVoters {
		// sanity check
		assert.Equal(t.t, markerAlias, fmt.Sprintf("%d,%d", t.StructureDetails(markerAlias).PastMarkers().Marker().SequenceID(), t.StructureDetails(markerAlias).PastMarkers().Marker().Index()))

		voters := t.SequenceTracker.Voters(t.StructureDetails(markerAlias).PastMarkers().Marker())

		assert.True(t.t, expectedVotersOfMarker.Equal(voters), "expected %s but got %s", expectedVotersOfMarker, voters)
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
