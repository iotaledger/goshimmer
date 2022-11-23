package votes

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test              *testing.T
	Validators        *sybilprotection.WeightedSet
	validatorsByAlias map[string]*validator.Validator
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:              test,
		Validators:        nil, /* TODO: FIX */
		validatorsByAlias: make(map[string]*validator.Validator),
	}, opts)
}

func (t *TestFramework) CreateValidator(alias string, opts ...options.Option[validator.Validator]) *validator.Validator {
	return t.CreateValidatorWithID(alias, lo.PanicOnErr(identity.RandomIDInsecure()), opts...)
}

func (t *TestFramework) CreateValidatorWithID(alias string, id identity.ID, opts ...options.Option[validator.Validator]) *validator.Validator {
	voter := validator.New(id, opts...)

	t.validatorsByAlias[alias] = voter
	t.Validators.Add(id)

	weightUpdates := sybilprotection.NewWeightUpdates(1)
	weightUpdates.ApplyDiff(id, voter.Weight())
	t.Validators.Weights.ApplyUpdates(weightUpdates)

	return voter
}

func (t *TestFramework) Validator(alias string) (v *validator.Validator) {
	v, ok := t.validatorsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Validator alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) ValidatorsSet(aliases ...string) (validators *set.AdvancedSet[*validator.Validator]) {
	validators = set.NewAdvancedSet[*validator.Validator]()
	for _, alias := range aliases {
		validators.Add(t.Validator(alias))
	}

	return
}

func ValidatorSetToAdvancedSet(validatorSet *validator.Set) (validatorAdvancedSet *set.AdvancedSet[*validator.Validator]) {
	validatorAdvancedSet = set.NewAdvancedSet[*validator.Validator]()
	validatorSet.ForEach(func(_ identity.ID, validator *validator.Validator) bool {
		validatorAdvancedSet.Add(validator)
		return true
	})
	return validatorAdvancedSet
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithValidators(validators *sybilprotection.WeightedSet) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.Validators = validators
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedVotePower //////////////////////////////////////////////////////////////////////////////////////////////

type MockedVotePower struct {
	VotePower int
}

func (p MockedVotePower) Compare(other MockedVotePower) int {
	if p.VotePower-other.VotePower < 0 {
		return -1
	} else if p.VotePower-other.VotePower > 0 {
		return 1
	} else {
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
