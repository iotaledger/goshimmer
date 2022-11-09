package votes

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	ValidatorSet *validator.Set

	test              *testing.T
	validatorsByAlias map[string]*validator.Validator
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:              test,
		validatorsByAlias: make(map[string]*validator.Validator),
	}, opts, func(t *TestFramework) {
		if t.ValidatorSet == nil {
			t.ValidatorSet = validator.NewSet()
		}
	})
}

func (t *TestFramework) CreateValidator(alias string, opts ...options.Option[validator.Validator]) *validator.Validator {
	return t.CreateValidatorWithID(alias, lo.PanicOnErr(identity.RandomIDInsecure()), opts...)
}

func (t *TestFramework) CreateValidatorWithID(alias string, id identity.ID, opts ...options.Option[validator.Validator]) *validator.Validator {
	voter := validator.New(id, opts...)

	t.validatorsByAlias[alias] = voter
	t.ValidatorSet.Add(voter)

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

func WithValidatorSet(validatorSet *validator.Set) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.ValidatorSet != nil {
			panic("validator set already set")
		}

		tf.ValidatorSet = validatorSet
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedVotePower //////////////////////////////////////////////////////////////////////////////////////////////

type MockedVotePower struct {
	VotePower int
}

func (p MockedVotePower) CompareTo(other MockedVotePower) int {
	if p.VotePower-other.VotePower < 0 {
		return -1
	} else if p.VotePower-other.VotePower > 0 {
		return 1
	} else {
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
