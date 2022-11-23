package sybilprotection

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
)

type SybilProtection interface {
	Weights() (weights *Weights)
	Validators() (validators *WeightedSet)

	InitModule()

	ledgerstate.DiffConsumer
}

/*
type MockSybilProtection struct {
	weights            *sybilprotection.Weights
	validators         *sybilprotection.WeightedSet
}

func (m *MockSybilProtection) Weights() (weights *Weights) {
	return m.weights
}

func (m *MockSybilProtection) Validators() (validators *WeightedSet) {
	return m.validators
}


*/