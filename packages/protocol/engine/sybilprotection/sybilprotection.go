package sybilprotection

import (
	"github.com/iotaledger/goshimmer/packages/core/traits"
)

type SybilProtection interface {
	Weights() (weights *Weights)
	Validators() (validators *WeightedSet)

	traits.Initializable
}
