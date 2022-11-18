package sybilprotection

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/weights"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type SybilProtection interface {
	Validators() (validators *weights.Set)
	Weights() (weightedActors *weights.Vector)
	Attestations(index epoch.Index) (attestations *models.Attestations)
	InitModule()
}
