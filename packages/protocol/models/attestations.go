package models

import (
	"sync"

	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/weights"
)

type Attestations struct {
	weightedSet *weights.Set
	storage     *memstorage.Storage[identity.ID, *memstorage.Storage[BlockID, *Attestation]]
	mutex       sync.RWMutex
}

func NewAttestations(weightsVector *weights.Vector) *Attestations {
	return &Attestations{
		weightedSet: weightsVector.NewWeightedSet(),
		storage:     memstorage.New[identity.ID, *memstorage.Storage[BlockID, *Attestation]](),
	}
}

func (a *Attestations) Add(block *Block) (added bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// TODO: CHECK IF PAST EVICTION

	storage, created := a.storage.RetrieveOrCreate(block.IssuerID(), memstorage.New[BlockID, *Attestation])
	if created {
		a.weightedSet.Add(block.IssuerID())
	}

	return storage.Set(block.ID(), NewAttestation(block))
}

func (a *Attestations) Delete(block *Block) (deleted bool) {
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

	return a.weightedSet.Weight()
}

func (a *Attestations) Detach() {
	a.weightedSet.Detach()
}
