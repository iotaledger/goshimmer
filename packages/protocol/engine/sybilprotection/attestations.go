package sybilprotection

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Attestations interface {
	Add(block *models.Block) (added bool)
	Delete(block *models.Block) (removed bool)
	AuthenticatedSet() (adsAttestors *ads.Set[identity.ID])
	TotalWeight() (totalWeight int64)
}
