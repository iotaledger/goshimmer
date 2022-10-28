package tangle

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/serix"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type ActivityLogStorage struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

type ActivityEntry struct {
	Index epoch.Index `serix:"0"`
	ID    identity.ID `serix:"1"`
}

func (a *ActivityEntry) FromBytes(bytes []byte) (int, error) {
	return serix.DefaultAPI.Decode(context.Background(), bytes, a)
}

func (a ActivityEntry) Bytes() ([]byte, error) {
	return serix.DefaultAPI.Encode(context.Background(), a)
}

func (s *ActivityLogStorage) Store(activityEntry *ActivityEntry) (err error) {
	idBytes := lo.PanicOnErr(activityEntry.ID.Bytes())
	if err = s.Storage(activityEntry.Index).Set(idBytes, idBytes); err != nil {
		return errors.Errorf("failed to store active id %s: %w", activityEntry.ID, err)
	}

	return nil
}

func (s *ActivityLogStorage) GetAll(index epoch.Index) (ids *set.AdvancedSet[identity.ID]) {
	ids = set.NewAdvancedSet[identity.ID]()
	s.Stream(index, func(id identity.ID) {
		ids.Add(id)
	})
	return
}

func (s *ActivityLogStorage) Delete(index epoch.Index, id identity.ID) (err error) {
	if err = s.Storage(index).Delete(lo.PanicOnErr(id.Bytes())); err != nil {
		return errors.Errorf("failed to delete active id %s: %w", id, err)
	}

	return nil
}

func (s *ActivityLogStorage) Stream(index epoch.Index, callback func(identity.ID)) {
	s.Storage(index).Iterate([]byte{}, func(idBytes kvstore.Key, _ kvstore.Value) bool {
		id := new(identity.ID)
		id.FromBytes(idBytes)
		callback(*id)
		return true
	})
}
