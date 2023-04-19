package test

import (
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
)

type Framework struct {
	Instance sybilprotection.SybilProtection

	identitiesByAlias map[string]identity.ID
}

func NewFramework(instance sybilprotection.SybilProtection) *Framework {
	return &Framework{
		Instance:          instance,
		identitiesByAlias: make(map[string]identity.ID),
	}
}

func (f *Framework) Weights() *sybilprotection.Weights {
	return f.Instance.Weights()
}

func (f *Framework) Validators() *sybilprotection.WeightedSet {
	return f.Instance.Validators()
}

func (f *Framework) CreateValidator(validatorAlias string, optWeight ...int64) error {
	validatorID, exists := f.identitiesByAlias[validatorAlias]
	if exists {
		return xerrors.Errorf("")
	}

	f.Instance.Weights().Update(validatorID, sybilprotection.NewWeight(lo.First(optWeight), 1))

	return nil
}

func (f *Framework) ValidatorID(alias string) (id identity.ID, exists bool) {
	id, exists = f.identitiesByAlias[alias]

	return id, exists
}