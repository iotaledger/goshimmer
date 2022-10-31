package prunable

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type ActivityLog struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

func (s *ActivityLog) Store(index epoch.Index, id identity.ID) (err error) {
	idBytes, err := id.Bytes()
	if err != nil {
		return errors.Errorf("failed to get id bytes: %w", err)
	}

	if err = s.Storage(index).Set(idBytes, idBytes); err != nil {
		return errors.Errorf("failed to store active id %s: %w", id, err)
	}

	return nil
}

func (s *ActivityLog) DeleteActivity(index epoch.Index, id identity.ID) (err error) {
	if err = s.Storage(index).Delete(lo.PanicOnErr(id.Bytes())); err != nil {
		return errors.Errorf("failed to delete active id %s: %w", id, err)
	}

	return nil
}

func (s *ActivityLog) Load(index epoch.Index) (ids *set.AdvancedSet[identity.ID]) {
	ids = set.NewAdvancedSet[identity.ID]()
	s.Stream(index, func(id identity.ID) {
		ids.Add(id)
	})
	return
}

func (s *ActivityLog) Stream(index epoch.Index, callback func(identity.ID)) {
	s.Storage(index).Iterate([]byte{}, func(idBytes kvstore.Key, _ kvstore.Value) bool {
		id := new(identity.ID)
		id.FromBytes(idBytes)
		callback(*id)
		return true
	})
}
