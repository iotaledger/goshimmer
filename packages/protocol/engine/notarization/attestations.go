package notarization

import (
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type Attestations struct {
	storage *storage.Storage

	traits.Initializable
}

func NewAttestations(storageInstance *storage.Storage) (newActiveNodes *Attestations) {
	return &Attestations{
		Initializable: traits.NewInitializable(),
		storage:       storageInstance,
	}
}

func (a *Attestations) Persist(attestations *EpochAttestations) (adsMap *ads.Map[identity.ID, Attestation, *identity.ID, *Attestation], err error) {
	adsMap = ads.NewMap[identity.ID, Attestation](a.storage.Prunable.Attestors(attestations.Epoch))
	attestations.ForEachUniqueAttestation(func(id identity.ID, attestation *Attestation) bool {
		adsMap.Set(id, attestation)
		return true
	})

	return
}

func (a *Attestations) Delete(index epoch.Index, id identity.ID) (err error) {
	if err = a.storage.Prunable.Attestors(index).Delete(lo.PanicOnErr(id.Bytes())); err != nil {
		return errors.Errorf("failed to delete active id %s: %w", id, err)
	}

	return nil
}

func (a *Attestations) LoadAll(index epoch.Index) (ids *set.AdvancedSet[identity.ID]) {
	ids = set.NewAdvancedSet[identity.ID]()
	a.Stream(index, func(id identity.ID) error {
		ids.Add(id)
		return nil
	})
	return
}

func (a *Attestations) Stream(index epoch.Index, callback func(attestor identity.ID) (error error)) (err error) {
	if iterationErr := a.storage.Prunable.Attestors(index).Iterate([]byte{}, func(idBytes kvstore.Key, _ kvstore.Value) bool {
		id := new(identity.ID)
		if _, err = id.FromBytes(idBytes); err != nil {
			err = errors.Errorf("failed to parse id bytes: %w", err)
		} else if err = callback(*id); err != nil {
			err = errors.Errorf("failed to process id %s: %w", *id, err)
		}

		return err != nil
	}); iterationErr != nil {
		err = errors.Errorf("failed to iterate over active ids: %w", iterationErr)
	}

	return err
}

func (a *Attestations) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if err = stream.Write(writer, uint64(targetEpoch)); err != nil {
		return errors.Errorf("failed to write epoch: %w", err)
	}

	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		if err = a.Stream(targetEpoch, func(id identity.ID) (err error) {
			if err = id.Export(writer); err == nil {
				elementsCount++
			}

			return
		}); err != nil {
			return 0, errors.Errorf("failed to stream attestors: %w", err)
		}

		return
	})
}

func (a *Attestations) Import(reader io.ReadSeeker) (err error) {
	epochIndex, err := stream.Read[uint64](reader)
	if err != nil {
		return errors.Errorf("failed to read epoch: %w", err)
	}

	attestor := new(identity.ID)
	return stream.ReadCollection(reader, func(i int) error {
		if err = attestor.Import(reader); err != nil {
			return errors.Errorf("failed to read attestor %d: %w", i, err)
		} else if err = a.Store(epoch.Index(epochIndex), *attestor); err != nil {
			return errors.Errorf("failed to store attestor %d with id %s: %w", i, attestor, err)
		}

		return nil
	})
}
