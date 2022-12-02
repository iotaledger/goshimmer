package notarization

import (
	"encoding/binary"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const (
	PrefixAttestationsLastCommittedEpoch byte = iota
	PrefixAttestationsWeight
	PrefixAttestationsStorage
)

type Attestations struct {
	persistentStorage  kvstore.KVStore
	bucketedStorage    func(index epoch.Index) kvstore.KVStore
	weights            *sybilprotection.Weights
	cachedAttestations *memstorage.EpochStorage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]]
	mutex              *syncutils.DAGMutex[epoch.Index]

	traits.Committable
}

func NewAttestations(persistentStorage kvstore.KVStore, bucketedStorage func(index epoch.Index) kvstore.KVStore, weights *sybilprotection.Weights) *Attestations {
	return &Attestations{
		persistentStorage:  persistentStorage,
		bucketedStorage:    bucketedStorage,
		weights:            weights,
		cachedAttestations: memstorage.NewEpochStorage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]](),
		mutex:              syncutils.NewDAGMutex[epoch.Index](),
	}
}

func (a *Attestations) Add(block *models.Block) (added bool, err error) {
	a.mutex.RLock(block.ID().Index())
	defer a.mutex.RUnlock(block.ID().Index())

	if block.ID().Index() <= a.LastCommittedEpoch() {
		return false, errors.Errorf("cannot add block %s to attestations: block is from past epoch", block.ID())
	}

	epochStorage, _ := a.cachedAttestations.RetrieveOrCreate(block.ID().Index(), func() bool {
		// TODO: PRUNE PERSISTENT STORAGE, in case of a crash, we might still have old data (or do it on startup)

		return true
	})

	issuerStorage, _ := epochStorage.RetrieveOrCreate(block.IssuerID(), memstorage.New[models.BlockID, *Attestation])

	return issuerStorage.Set(block.ID(), NewAttestation(block)), nil
}

func (a *Attestations) Delete(block *models.Block) (deleted bool, err error) {
	a.mutex.RLock(block.ID().Index())
	defer a.mutex.RUnlock(block.ID().Index())

	if block.ID().Index() <= a.LastCommittedEpoch() {
		return false, errors.Errorf("cannot add block %s to attestations: block is from past epoch", block.ID())
	}

	epochStorage := a.cachedAttestations.Get(block.ID().Index(), false)
	if epochStorage == nil {
		return false, errors.Errorf("cannot delete block %s from attestations: no attestations for epoch %d", block.ID(), block.ID().Index())
	}

	issuerStorage, exists := epochStorage.Get(block.IssuerID())
	if !exists {
		return false, errors.Errorf("cannot delete block %s from attestations: no attestations for issuer %s", block.ID(), block.IssuerID())
	}

	return issuerStorage.Delete(block.ID()), nil
}

func (a *Attestations) Commit(index epoch.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], weight int64, err error) {
	a.mutex.Lock(index)
	defer a.mutex.Unlock(index)

	if attestations, weight, err = a.commit(index); err != nil {
		return nil, 0, errors.Errorf("failed to commit attestations for epoch %d: %w", index, err)
	}

	if err = a.setWeight(index, weight); err != nil {
		return nil, 0, errors.Errorf("failed to commit attestations for epoch %d: %w", index, err)
	}

	if err = a.persistentStorage.Set([]byte{PrefixAttestationsLastCommittedEpoch}, index.Bytes()); err != nil {
		return nil, 0, errors.Errorf("failed to persist last committed epoch: %w", err)
	}

	if err = a.flushEpoch(index); err != nil {
		return nil, 0, errors.Errorf("failed to flush attestations for epoch %d: %w", index, err)
	}

	a.Committable.SetLastCommittedEpoch(index)

	return
}

func (a *Attestations) Weight(index epoch.Index) (weight int64, err error) {
	a.mutex.RLock(index)
	defer a.mutex.RUnlock(index)

	if index > a.LastCommittedEpoch() {
		return 0, errors.Errorf("cannot compute weight of attestations for epoch %d: epoch is not committed yet", index)
	}

	return a.weight(index)
}

func (a *Attestations) Attestations(index epoch.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], err error) {
	a.mutex.RLock(index)
	defer a.mutex.RUnlock(index)

	if index > a.LastCommittedEpoch() {
		return nil, errors.Errorf("cannot retrieve attestations for epoch %d: epoch is not committed yet", index)
	}

	return a.attestations(index)
}

func (a *Attestations) commit(index epoch.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], weight int64, err error) {
	if attestations, err = a.attestations(index); err != nil {
		return nil, 0, errors.Errorf("failed to get attestors for epoch %d: %w", index, err)
	}

	if cachedEpochStorage := a.cachedAttestations.Evict(index); cachedEpochStorage != nil {
		cachedEpochStorage.ForEach(func(id identity.ID, attestationsOfID *memstorage.Storage[models.BlockID, *Attestation]) bool {
			if latestAttestation := latestAttestation(attestationsOfID); latestAttestation != nil {
				if attestorWeight, exists := a.weights.Weight(id); exists {
					attestations.Set(id, latestAttestation)

					weight += attestorWeight.Value
				}
			}

			return true
		})
	}

	return
}

func (a *Attestations) flushEpoch(index epoch.Index) (err error) {
	if err = a.persistentStorage.Flush(); err != nil {
		return errors.Errorf("failed to flush persistent storage: %w", err)
	}

	if err = a.bucketedStorage(index).Flush(); err != nil {
		return errors.Errorf("failed to flush attestations for epoch %d: %w", index, err)
	}

	return
}

func (a *Attestations) attestations(index epoch.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], err error) {
	if attestationsStorage, err := a.bucketedStorage(index).WithExtendedRealm([]byte{PrefixAttestationsStorage}); err != nil {
		return nil, errors.Errorf("failed to access storage for attestors of epoch %d: %w", index, err)
	} else {
		return ads.NewMap[identity.ID, Attestation, *identity.ID, *Attestation](attestationsStorage), nil
	}
}

func (a *Attestations) weight(index epoch.Index) (weight int64, err error) {
	if value, err := a.bucketedStorage(index).Get([]byte{PrefixAttestationsWeight}); err != nil {
		return 0, errors.Errorf("failed to retrieve weight of attestations for epoch %d: %w", index, err)
	} else {
		return int64(binary.LittleEndian.Uint64(value)), nil
	}
}

func (a *Attestations) setWeight(index epoch.Index, weight int64) (err error) {
	weightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(weightBytes, uint64(weight))

	if err = a.bucketedStorage(index).Set([]byte{PrefixAttestationsWeight}, weightBytes); err != nil {
		return errors.Errorf("failed to store weight of attestations for epoch %d: %w", index, err)
	}

	return
}

func latestAttestation(attestations *memstorage.Storage[models.BlockID, *Attestation]) (latestAttestation *Attestation) {
	attestations.ForEach(func(blockID models.BlockID, attestation *Attestation) bool {
		if attestation.Compare(latestAttestation) > 0 {
			latestAttestation = attestation
		}

		return true
	})

	return
}
