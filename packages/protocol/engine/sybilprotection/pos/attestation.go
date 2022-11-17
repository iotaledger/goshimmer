package pos

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Attestations struct {
	weights *memstorage.Storage[identity.ID, int64]
	storage *memstorage.Storage[identity.ID, *memstorage.Storage[models.BlockID, *models.Attestation]]
}

func NewEpochAttestations(weights *memstorage.Storage[identity.ID, int64]) *Attestations {
	return &Attestations{
		weights: weights,
		storage: memstorage.New[identity.ID, *memstorage.Storage[models.BlockID, *models.Attestation]](),
	}
}

func (a *Attestations) TotalWeight() (totalWeight int64) {
	if a == nil {
		return 0
	}

	a.storage.ForEachKey(func(attestor identity.ID) bool {
		totalWeight += lo.Return1(a.weights.Get(attestor))

		return true
	})

	return
}

func (a *Attestations) AuthenticatedSet() (adsAttestors *ads.Set[identity.ID]) {
	adsAttestors = ads.NewSet[identity.ID](mapdb.NewMapDB())

	if a == nil {
		return
	}

	a.storage.ForEachKey(func(attestor identity.ID) bool {
		adsAttestors.Add(attestor)

		return true
	})

	return
}

func (a *Attestations) Add(block *models.Block) (added bool) {
	return lo.Return1(a.storage.RetrieveOrCreate(block.IssuerID(), memstorage.New[models.BlockID, *models.Attestation])).Set(
		block.ID(), models.NewAttestation(block),
	)
}

func (a *Attestations) Delete(block *models.Block) (deleted bool) {
	if storage, exists := a.storage.Get(block.IssuerID()); exists {
		if deleted = storage.Delete(block.ID()); deleted && storage.IsEmpty() {
			a.storage.Delete(block.IssuerID())
		}
	}

	return
}
