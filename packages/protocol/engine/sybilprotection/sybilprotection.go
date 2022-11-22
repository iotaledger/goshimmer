package sybilprotection

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/state"
)

type SybilProtection interface {
	Weights() (weights *Weights)
	Validators() (validators *WeightedSet)

	InitModule()
	state.Consumer
}
