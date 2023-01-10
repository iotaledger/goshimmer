package notarization

import (
	"sync"

	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Attestations struct {
	weightedSet *sybilprotection.WeightedSet
	storage     *memstorage.Storage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]]
	mutex       sync.RWMutex
}

func NewAttestations(weights *sybilprotection.Weights) *Attestations {
	return &Attestations{
		weightedSet: weights.WeightedSet(),
		storage:     memstorage.New[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]](),
	}
}

func (a *Attestations) Add(block *models.Block) (added bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// TODO: CHECK IF PAST EVICTION

	storage, created := a.storage.RetrieveOrCreate(block.IssuerID(), memstorage.New[models.BlockID, *Attestation])
	if created {
		a.weightedSet.Add(block.IssuerID())
	}

	return storage.Set(block.ID(), NewAttestation(block))
}

func (a *Attestations) Delete(block *models.Block) (deleted bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if storage, exists := a.storage.Get(block.IssuerID()); exists {
		if deleted = storage.Delete(block.ID()); deleted && storage.IsEmpty() {
			a.weightedSet.Delete(block.IssuerID())
			a.storage.Delete(block.IssuerID())
		}
	}

	return
}

func (a *Attestations) Attestors() (attestors *ads.Set[identity.ID]) {
	attestors = ads.NewSet[identity.ID](mapdb.NewMapDB())

	if a == nil {
		return
	}

	a.mutex.RLock()
	defer a.mutex.RUnlock()

	_ = a.weightedSet.ForEach(func(attestor identity.ID) error {
		attestors.Add(attestor)
		return nil
	})

	return
}

func (a *Attestations) Weight() (weight int64) {
	if a == nil {
		return 0
	}

	return a.weightedSet.TotalWeight()
}

func (a *Attestations) Detach() {
	a.weightedSet.Detach()
}
