package otv

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
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type TestFramework struct {
	VotesTracker *VotesTracker[utxo.TransactionID, utxo.OutputID]
	validatorSet *validator.Set

	validatorsByAlias map[string]*validator.Validator

	t *testing.T

	*conflictdag.TestFramework
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(t *testing.T) (newFramework *TestFramework) {
	conflictDAGTf := conflictdag.NewTestFramework(t)
	validatorSet := validator.NewSet()
	return &TestFramework{
		VotesTracker:      NewVotesTracker[utxo.TransactionID, utxo.OutputID](conflictDAGTf.ConflictDAG, validatorSet),
		TestFramework:     conflictDAGTf,
		validatorSet:      validatorSet,
		validatorsByAlias: make(map[string]*validator.Validator),
		t:                 t,
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

type MockVotePower struct {
	votePower int
}

func (p MockVotePower) CompareTo(other VotePower) int {
	if specificOther, ok := other.(MockVotePower); ok {
		return p.votePower - specificOther.votePower
	}

	return 1
}
