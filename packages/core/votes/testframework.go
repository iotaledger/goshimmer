package votes

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test              *testing.T
	Validators        *sybilprotection.WeightedSet
	validatorsByAlias map[string]identity.ID
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test:              test,
		Validators:        nil,
		validatorsByAlias: make(map[string]identity.ID),
	}, opts)
}

func (t *TestFramework) CreateValidator(alias string, weight int64) {
	t.CreateValidatorWithID(alias, lo.PanicOnErr(identity.RandomIDInsecure()), weight)
}

func (t *TestFramework) CreateValidatorWithID(alias string, id identity.ID, weight int64, skipWeightUpdate ...bool) {
	t.validatorsByAlias[alias] = id

	if len(skipWeightUpdate) == 1 && skipWeightUpdate[0] {
		return
	}
	t.Validators.Weights.Update(id, sybilprotection.NewWeight(weight, 0))
	t.Validators.Add(id)
}

func (t *TestFramework) Validator(alias string) (v identity.ID) {
	v, ok := t.validatorsByAlias[alias]
	if !ok {
		panic(fmt.Sprintf("Validator alias %s not registered", alias))
	}

	return
}

func (t *TestFramework) ValidatorsSet(aliases ...string) (validators *set.AdvancedSet[identity.ID]) {
	validators = set.NewAdvancedSet[identity.ID]()
	for _, alias := range aliases {
		validators.Add(t.Validator(alias))
	}

	return
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
