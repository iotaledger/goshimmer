package notarization

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const (
	PrefixAttestations byte = iota
	PrefixAttestationsLastCommittedEpoch
	PrefixAttestationsWeight
)

type Attestations struct {
	persistentStorage  func(optRealm ...byte) kvstore.KVStore
	bucketedStorage    func(index epoch.Index) kvstore.KVStore
	weights            *sybilprotection.Weights
	cachedAttestations *memstorage.EpochStorage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]]
	mutex              *syncutils.DAGMutex[epoch.Index]

	traits.Initializable
	traits.Committable
}

func NewAttestations(persistentStorage func(optRealm ...byte) kvstore.KVStore, bucketedStorage func(index epoch.Index) kvstore.KVStore, weights *sybilprotection.Weights) *Attestations {
	return &Attestations{
		Committable:        traits.NewCommittable(persistentStorage(), PrefixAttestationsLastCommittedEpoch),
		Initializable:      traits.NewInitializable(),
		persistentStorage:  persistentStorage,
		bucketedStorage:    bucketedStorage,
		weights:            weights,
		cachedAttestations: memstorage.NewEpochStorage[identity.ID, *memstorage.Storage[models.BlockID, *Attestation]](),
		mutex:              syncutils.NewDAGMutex[epoch.Index](),
	}
}

func (a *Attestations) Add(attestation *Attestation) (added bool, err error) {
	epochIndex := epoch.IndexFromTime(attestation.IssuingTime)

	a.mutex.RLock(epochIndex)
	defer a.mutex.RUnlock(epochIndex)

	if epochIndex <= a.LastCommittedEpoch() {
		return false, errors.Errorf("cannot add attestation: block is from past epoch")
	}

	epochStorage := a.cachedAttestations.Get(epochIndex, true)
	issuerStorage, _ := epochStorage.RetrieveOrCreate(attestation.IssuerID, memstorage.New[models.BlockID, *Attestation])

	return issuerStorage.Set(attestation.ID(), attestation), nil
}

func (a *Attestations) Delete(attestation *Attestation) (deleted bool, err error) {
	epochIndex := epoch.IndexFromTime(attestation.IssuingTime)

	a.mutex.RLock(epochIndex)
	defer a.mutex.RUnlock(epochIndex)

	if epochIndex <= a.LastCommittedEpoch() {
		return false, errors.Errorf("cannot delete attestation from past epoch %d", epochIndex)
	}

	epochStorage := a.cachedAttestations.Get(epochIndex, false)
	if epochStorage == nil {
		return false, nil
	}

	issuerStorage, exists := epochStorage.Get(attestation.IssuerID)
	if !exists {
		return false, nil
	}

	return issuerStorage.Delete(attestation.ID()), nil
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

	a.SetLastCommittedEpoch(index)

	if err = a.flush(index); err != nil {
		return nil, 0, errors.Errorf("failed to flush attestations for epoch %d: %w", index, err)
	}

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

func (a *Attestations) Get(index epoch.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], err error) {
	a.mutex.RLock(index)
	defer a.mutex.RUnlock(index)

	if index > a.LastCommittedEpoch() {
		return nil, errors.Errorf("cannot retrieve attestations for epoch %d: epoch is not committed yet", index)
	}

	return a.attestations(index)
}

func (a *Attestations) Import(reader io.ReadSeeker) (err error) {
	epochIndex, err := stream.Read[uint64](reader)
	if err != nil {
		return errors.Errorf("failed to read epoch: %w", err)
	}

	weight, err := stream.Read[int64](reader)
	if err != nil {
		return errors.Errorf("failed to read weight for epoch: %w", err)
	}

	attestations, err := a.attestations(epoch.Index(epochIndex))
	if err != nil {
		return errors.Errorf("failed to import attestations for epoch %d: %w", epochIndex, err)
	}

	importedAttestation := new(Attestation)
	if err = stream.ReadCollection(reader, func(i int) (err error) {
		if err = stream.ReadSerializable(reader, importedAttestation); err != nil {
			return errors.Errorf("failed to read attestation %d: %w", i, err)
		}

		attestations.Set(importedAttestation.IssuerID, importedAttestation)

		return
	}); err != nil {
		return errors.Errorf("failed to import attestations for epoch %d: %w", epochIndex, err)
	}

	if err = a.setWeight(epoch.Index(epochIndex), weight); err != nil {
		return errors.Errorf("failed to set attestations weight of epoch %d: %w", epochIndex, err)
	}

	a.SetLastCommittedEpoch(epoch.Index(epochIndex))

	a.TriggerInitialized()

	return
}

func (a *Attestations) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if err = stream.Write(writer, uint64(targetEpoch)); err != nil {
		return errors.Errorf("failed to write epoch: %w", err)
	}

	if weight, err := a.weight(targetEpoch); targetEpoch != 0 && err != nil {
		return errors.Errorf("failed to obtain weight for epoch: %w", err)
	} else if err = stream.Write(writer, weight); err != nil {
		return errors.Errorf("failed to write epoch weight: %w", err)
	}

	return stream.WriteCollection(writer, func() (elementsCount uint64, writeErr error) {
		attestations, writeErr := a.attestations(targetEpoch)
		if writeErr != nil {
			return 0, errors.Errorf("failed to export attestations for epoch %d: %w", targetEpoch, writeErr)
		}

		if streamErr := attestations.Stream(func(issuerID identity.ID, attestation *Attestation) bool {
			if writeErr = stream.WriteSerializable(writer, attestation); writeErr != nil {
				writeErr = errors.Errorf("failed to write attestation for issuer %s: %w", issuerID, writeErr)
			} else {
				elementsCount++
			}

			return writeErr == nil
		}); streamErr != nil {
			return 0, errors.Errorf("failed to stream attestations of epoch %d: %w", targetEpoch, streamErr)
		}

		return
	})
}

func (a *Attestations) commit(index epoch.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], weight int64, err error) {
	if attestations, err = a.attestations(index); err != nil {
		return nil, 0, errors.Errorf("failed to get attestors for epoch %d: %w", index, err)
	}

	if cachedEpochStorage := a.cachedAttestations.Evict(index); cachedEpochStorage != nil {
		cachedEpochStorage.ForEach(func(id identity.ID, attestationsOfID *memstorage.Storage[models.BlockID, *Attestation]) bool {
			if latestAttestation := latestAttestation(attestationsOfID); latestAttestation != nil {
				if attestorWeight, exists := a.weights.Get(id); exists {
					attestations.Set(id, latestAttestation)

					weight += attestorWeight.Value
				}
			}

			return true
		})
	}

	return
}

func (a *Attestations) flush(index epoch.Index) (err error) {
	if err = a.persistentStorage().Flush(); err != nil {
		return errors.Errorf("failed to flush persistent storage: %w", err)
	}

	if err = a.bucketedStorage(index).Flush(); err != nil {
		return errors.Errorf("failed to flush attestations for epoch %d: %w", index, err)
	}

	return
}

func (a *Attestations) attestations(index epoch.Index) (attestations *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], err error) {
	if attestationsStorage, err := a.bucketedStorage(index).WithExtendedRealm([]byte{PrefixAttestations}); err != nil {
		return nil, errors.Errorf("failed to access storage for attestors of epoch %d: %w", index, err)
	} else {
		return ads.NewMap[identity.ID, Attestation](attestationsStorage), nil
	}
}

func (a *Attestations) weight(index epoch.Index) (weight int64, err error) {
	if value, err := a.bucketedStorage(index).Get([]byte{PrefixAttestationsWeight}); err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return 0, nil
		}

		return 0, errors.Errorf("failed to retrieve weight of attestations for epoch %d: %w", index, err)
	} else {
		return int64(binary.LittleEndian.Uint64(value)), nil
	}
}

func (a *Attestations) setWeight(index epoch.Index, weight int64) (err error) {
	weightBytes := make([]byte, marshalutil.Uint64Size)
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
