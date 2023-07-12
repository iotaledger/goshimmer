package sybilprotection

import (
	"testing"

	"golang.org/x/xerrors"

	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
)

type WeightedSetTestFramework struct {
	Instance *WeightedSet

	test              *testing.T
	identitiesByAlias map[string]identity.ID
}

func NewWeightedSetTestFramework(test *testing.T, instance *WeightedSet) *WeightedSetTestFramework {
	return &WeightedSetTestFramework{
		Instance:          instance,
		test:              test,
		identitiesByAlias: make(map[string]identity.ID),
	}
}

func (f *WeightedSetTestFramework) Add(alias string) bool {
	validatorID, exists := f.identitiesByAlias[alias]
	if !exists {
		f.test.Fatal(xerrors.Errorf("identity with alias '%s' does not exist", alias))
	}

	return f.Instance.Add(validatorID)
}

func (f *WeightedSetTestFramework) Delete(alias string) bool {
	validatorID, exists := f.identitiesByAlias[alias]
	if !exists {
		f.test.Fatal(xerrors.Errorf("identity with alias '%s' does not exist", alias))
	}

	return f.Instance.Delete(validatorID)
}

func (f *WeightedSetTestFramework) Get(alias string) (weight *Weight, exists bool) {
	validatorID, exists := f.identitiesByAlias[alias]
	if !exists {
		f.test.Fatal(xerrors.Errorf("identity with alias '%s' does not exist", alias))
	}

	return f.Instance.Get(validatorID)
}

func (f *WeightedSetTestFramework) Has(alias string) bool {
	validatorID, exists := f.identitiesByAlias[alias]
	if !exists {
		f.test.Fatal(xerrors.Errorf("identity with alias '%s' does not exist", alias))
	}

	return f.Instance.Has(validatorID)
}

func (f *WeightedSetTestFramework) ForEach(callback func(id identity.ID) error) (err error) {
	return f.Instance.ForEach(callback)
}

func (f *WeightedSetTestFramework) ForEachWeighted(callback func(id identity.ID, weight int64) error) (err error) {
	return f.Instance.ForEachWeighted(callback)
}

func (f *WeightedSetTestFramework) TotalWeight() int64 {
	return f.Instance.TotalWeight()
}

func (f *WeightedSetTestFramework) CreateID(alias string, optWeight ...int64) identity.ID {
	validatorID, exists := f.identitiesByAlias[alias]
	if exists {
		f.test.Fatal(xerrors.Errorf("identity with alias '%s' already exists", alias))
	}

	validatorID = identity.GenerateIdentity().ID()
	f.identitiesByAlias[alias] = validatorID

	f.Instance.Weights.Update(validatorID, NewWeight(lo.First(optWeight), 1))
	f.Instance.Add(validatorID)

	return validatorID
}

func (f *WeightedSetTestFramework) ID(alias string) identity.ID {
	id, exists := f.identitiesByAlias[alias]
	if !exists {
		f.test.Fatal(xerrors.Errorf("identity with alias '%s' does not exist", alias))
	}

	return id
}
