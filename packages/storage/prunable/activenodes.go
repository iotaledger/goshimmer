package prunable

import (
	"encoding/binary"
	"io"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/stream"
)

type Attestors struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

func NewAttestors(database *database.Manager, storagePrefix byte) (newActiveNodes *Attestors) {
	return &Attestors{
		Storage: lo.Bind([]byte{storagePrefix}, database.Get),
	}
}

func (a *Attestors) Store(index epoch.Index, id identity.ID) (err error) {
	idBytes, err := id.Bytes()
	if err != nil {
		return errors.Errorf("failed to get id bytes: %w", err)
	}

	if err = a.Storage(index).Set(idBytes, idBytes); err != nil {
		return errors.Errorf("failed to store active id %s: %w", id, err)
	}

	return nil
}

func (a *Attestors) Delete(index epoch.Index, id identity.ID) (err error) {
	if err = a.Storage(index).Delete(lo.PanicOnErr(id.Bytes())); err != nil {
		return errors.Errorf("failed to delete active id %s: %w", id, err)
	}

	return nil
}

func (a *Attestors) LoadAll(index epoch.Index) (ids *set.AdvancedSet[identity.ID]) {
	ids = set.NewAdvancedSet[identity.ID]()
	a.Stream(index, func(id identity.ID) error {
		ids.Add(id)
		return nil
	})
	return
}

func (a *Attestors) Stream(index epoch.Index, callback func(identity.ID) error) (err error) {
	if iterationErr := a.Storage(index).Iterate([]byte{}, func(idBytes kvstore.Key, _ kvstore.Value) bool {
		id := new(identity.ID)

		if _, err = id.FromBytes(idBytes); err != nil {
			err = errors.Errorf("failed to parse id bytes: %w", err)
		} else if err = callback(*id); err != nil {
			err = errors.Errorf("failed to process id %s: %w", *id, err)
		}

		return err != nil
	}); iterationErr != nil {
		return errors.Errorf("failed to iterate over active ids: %w", iterationErr)
	}

	return err
}

func (a *Attestors) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if err = binary.Write(writer, binary.LittleEndian, uint64(targetEpoch)); err != nil {
		return errors.Errorf("failed to write epoch: %w", err)
	}

	return stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		if err = a.Stream(targetEpoch, func(id identity.ID) (err error) {
			if err = id.Export(writer); err != nil {
				return errors.Errorf("failed to write id %s: %w", id, err)
			}

			elementsCount++

			return
		}); err != nil {
			return 0, errors.Errorf("failed to stream attestors: %w", err)
		}

		return elementsCount, nil
	})
}

func (a *Attestors) Import(reader io.ReadSeeker) (err error) {
	var targetEpochUint64 uint64
	if err = binary.Read(reader, binary.LittleEndian, &targetEpochUint64); err != nil {
		return errors.Errorf("failed to read epoch: %w", err)
	}

	return stream.ReadCollection(reader, func(i int) error {
		id := new(identity.ID)
		if err = id.Import(reader); err != nil {
			return errors.Errorf("failed to read attestor %d: %w", i, err)
		} else if err = a.Store(epoch.Index(targetEpochUint64), *id); err != nil {
			return errors.Errorf("failed to store attestor %d with id %s: %w", i, id, err)
		}

		return nil
	})
}
