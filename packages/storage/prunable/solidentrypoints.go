package prunable

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type EntryPoints struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

func NewSolidEntryPoints(database *database.Manager, storagePrefix byte) (newSolidEntryPoints *EntryPoints) {
	return &EntryPoints{
		Storage: lo.Bind([]byte{storagePrefix}, database.Get),
	}
}

func (s *EntryPoints) Store(id models.BlockID) (err error) {
	if err = s.Storage(id.Index()).Set(lo.PanicOnErr(id.Bytes()), lo.PanicOnErr(id.Bytes())); err != nil {
		return errors.Errorf("failed to store solid entry point block %s: %w", id, err)
	}

	return nil
}

func (s *EntryPoints) Delete(blockID models.BlockID) (err error) {
	if err = s.Storage(blockID.Index()).Delete(lo.PanicOnErr(blockID.Bytes())); err != nil {
		return errors.Errorf("failed to delete solid entry point block %s: %w", blockID, err)
	}

	return nil
}

func (s *EntryPoints) LoadAll(index epoch.Index) (solidEntryPoints *set.AdvancedSet[models.BlockID]) {
	solidEntryPoints = set.NewAdvancedSet[models.BlockID]()
	s.Stream(index, func(id models.BlockID) {
		solidEntryPoints.Add(id)
	})
	return
}

func (s *EntryPoints) Stream(index epoch.Index, callback func(models.BlockID)) {
	s.Storage(index).Iterate([]byte{}, func(blockIDBytes kvstore.Key, blockBytes kvstore.Value) bool {
		blockID := new(models.BlockID)
		blockID.FromBytes(blockIDBytes)
		callback(*blockID)
		return true
	})
}
