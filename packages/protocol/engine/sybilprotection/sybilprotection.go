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
