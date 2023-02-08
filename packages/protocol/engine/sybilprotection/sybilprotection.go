package sybilprotection

import (
	"github.com/iotaledger/goshimmer/packages/core/traits"
)

// SybilProtection is the minimal interface for the SybilProtection component of the IOTA protocol.
type SybilProtection interface {
	// Weights returns the weights of identities in the SybilProtection.
	Weights() (weights *Weights)

	// Validators returns the set of online validators that is used to track acceptance.
	Validators() (validators *WeightedSet)

	// Initializable exposes a subscribable life-cycle event that is triggered when the component is initialized.
	traits.Initializable

	// Committable is a trait that stores information about the latest commitment.
	traits.Committable
}
