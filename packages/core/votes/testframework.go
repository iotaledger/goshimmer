package votes

import (
	"fmt"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"testing"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/lo"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test              *testing.T
	Validators        *sybilprotection.WeightedSet
	validatorsByAlias map[string]identity.ID
}

// NewTestFramework is the constructor of the TestFramework.
func NewTestFramework(test *testing.T, validators *sybilprotection.WeightedSet) *TestFramework {
	return &TestFramework{
		test:              test,
		Validators:        validators,
		validatorsByAlias: make(map[string]identity.ID),
	}
}

func (t *TestFramework) CreateValidator(alias string, weight int64) {
	t.CreateValidatorWithID(alias, lo.PanicOnErr(identity.RandomIDInsecure()), weight)
}

func (t *TestFramework) CreateValidatorWithID(alias string, id identity.ID, weight int64) {
	t.validatorsByAlias[alias] = id

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

func (t *TestFramework) ValidatorsSet(aliases ...string) (validators *advancedset.AdvancedSet[identity.ID]) {
	validators = advancedset.NewAdvancedSet[identity.ID]()
	for _, alias := range aliases {
		validators.Add(t.Validator(alias))
	}

	return
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
