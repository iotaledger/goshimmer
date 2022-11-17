package types

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type SybilProtection interface {
	WeightedActors() (weightedActors WeightedActors)
	ActiveValidators() (activeValidators ActiveValidators)
	Attestations(index epoch.Index) (attestations Attestations)
}
