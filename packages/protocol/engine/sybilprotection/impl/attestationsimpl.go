package impl

import (
	"sync"

	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/generictypes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/types"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type AttestationsImpl struct {
	weightedSet *WeightedSet
	storage     *memstorage.Storage[identity.ID, *memstorage.Storage[models.BlockID, *generictypes.Attestation]]
	mutex       sync.RWMutex
}

func NewAttestations(weightedActors types.WeightedActors) *AttestationsImpl {
	return &AttestationsImpl{
		weightedSet: weightedActors.NewWeightedSet(),
		storage:     memstorage.New[identity.ID, *memstorage.Storage[models.BlockID, *generictypes.Attestation]](),
	}
}

func (a *AttestationsImpl) Add(block *models.Block) (added bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// TODO: CHECK IF PAST EVICTION

	storage, created := a.storage.RetrieveOrCreate(block.IssuerID(), memstorage.New[models.BlockID, *generictypes.Attestation])
	if created {
		a.weightedSet.Add(block.IssuerID())
	}

	return storage.Set(block.ID(), generictypes.NewAttestation(block))
}

func (a *AttestationsImpl) Delete(block *models.Block) (deleted bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if storage, exists := a.storage.Get(block.IssuerID()); exists {
		if deleted = storage.Delete(block.ID()); deleted && storage.IsEmpty() {
			a.weightedSet.Remove(block.IssuerID())
			a.storage.Delete(block.IssuerID())
		}
	}

	return
}

func (a *AttestationsImpl) Attestors() (attestors *ads.Set[identity.ID]) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	_ = a.weightedSet.ForEach(func(attestor identity.ID) error {
		attestors.Add(attestor)
		return nil
	})

	return
}

func (a *AttestationsImpl) Weight() (weight uint64) {
	if a == nil {
		return 0
	}

	return a.weightedSet.Weight()
}

func (a *AttestationsImpl) Dispose() {
	a.weightedSet.Dispose()
}
