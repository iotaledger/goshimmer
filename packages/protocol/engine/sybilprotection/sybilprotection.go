package sybilprotection

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type SybilProtection interface {
	Init()
	WeightsVector() (weightedActors WeightsVector)
	ActiveValidators() (activeValidators ActiveValidators)
	Attestations(index epoch.Index) (attestations Attestations)
}
