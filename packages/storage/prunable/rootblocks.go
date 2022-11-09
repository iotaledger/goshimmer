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

type RootBlocks struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

func NewRootBlocks(database *database.Manager, storagePrefix byte) (newRootBlocks *RootBlocks) {
	return &RootBlocks{
		Storage: lo.Bind([]byte{storagePrefix}, database.Get),
	}
}

func (r *RootBlocks) Store(id models.BlockID) (err error) {
	if err = r.Storage(id.Index()).Set(lo.PanicOnErr(id.Bytes()), []byte{1}); err != nil {
		return errors.Errorf("failed to store solid entry point block %s: %w", id, err)
	}

	return nil
}

func (r *RootBlocks) Has(blockID models.BlockID) (has bool, err error) {
	has, err = r.Storage(blockID.Index()).Has(lo.PanicOnErr(blockID.Bytes()))
	if err != nil {
		return false, errors.Errorf("failed to delete solid entry point block %s: %w", blockID, err)
	}

	return has, nil
}

func (r *RootBlocks) Delete(blockID models.BlockID) (err error) {
	if err = r.Storage(blockID.Index()).Delete(lo.PanicOnErr(blockID.Bytes())); err != nil {
		return errors.Errorf("failed to delete solid entry point block %s: %w", blockID, err)
	}

	return nil
}

func (r *RootBlocks) LoadAll(index epoch.Index) (solidEntryPoints *set.AdvancedSet[models.BlockID]) {
	solidEntryPoints = set.NewAdvancedSet[models.BlockID]()
	r.Stream(index, func(id models.BlockID) {
		solidEntryPoints.Add(id)
	})
	return
}

func (r *RootBlocks) StoreAll(solidEntryPoints *set.AdvancedSet[models.BlockID]) {
	for it := solidEntryPoints.Iterator(); it.HasNext(); {
		r.Store(it.Next())
	}
}

func (r *RootBlocks) Stream(index epoch.Index, callback func(models.BlockID)) {
	r.Storage(index).Iterate([]byte{}, func(blockIDBytes kvstore.Key, _ kvstore.Value) bool {
		blockID := new(models.BlockID)
		blockID.FromBytes(blockIDBytes)
		callback(*blockID)
		return true
	})
}
