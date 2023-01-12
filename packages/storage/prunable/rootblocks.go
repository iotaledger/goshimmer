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

// NewRootBlocks creates a new RootBlocks instance.
func NewRootBlocks(databaseInstance *database.Manager, storagePrefix byte) (newRootBlocks *RootBlocks) {
	return &RootBlocks{
		Storage: lo.Bind([]byte{storagePrefix}, databaseInstance.Get),
	}
}

// Store stores the given blockID as a root block.
func (r *RootBlocks) Store(id models.BlockID) (err error) {
	if err = r.Storage(id.Index()).Set(lo.PanicOnErr(id.Bytes()), []byte{1}); err != nil {
		return errors.Errorf("failed to store solid entry point block %s: %w", id, err)
	}

	return nil
}

// Has returns true if the given blockID is a root block.
func (r *RootBlocks) Has(blockID models.BlockID) (has bool, err error) {
	has, err = r.Storage(blockID.Index()).Has(lo.PanicOnErr(blockID.Bytes()))
	if err != nil {
		return false, errors.Errorf("failed to delete solid entry point block %s: %w", blockID, err)
	}

	return has, nil
}

// Delete deletes the given blockID from the root blocks.
func (r *RootBlocks) Delete(blockID models.BlockID) (err error) {
	if err = r.Storage(blockID.Index()).Delete(lo.PanicOnErr(blockID.Bytes())); err != nil {
		return errors.Errorf("failed to delete solid entry point block %s: %w", blockID, err)
	}

	return nil
}

// LoadAll loads all root blocks for an epoch index.
func (r *RootBlocks) LoadAll(index epoch.Index) (solidEntryPoints *set.AdvancedSet[models.BlockID]) {
	solidEntryPoints = set.NewAdvancedSet[models.BlockID]()
	if err := r.Stream(index, func(id models.BlockID) error {
		solidEntryPoints.Add(id)
		return nil
	}); err != nil {
		panic(errors.Errorf("failed to load all rootblocks for epoch %d: %w", index, err))
	}
	return
}

// StoreAll stores all passed root blocks.
func (r *RootBlocks) StoreAll(rootBlocks *set.AdvancedSet[models.BlockID]) (err error) {
	for it := rootBlocks.Iterator(); it.HasNext(); {
		if err := r.Store(it.Next()); err != nil {
			return errors.Errorf("failed to store rootblocks: %w", err)
		}
	}
	return nil
}

// Stream streams all root blocks for an epoch index.
func (r *RootBlocks) Stream(index epoch.Index, processor func(models.BlockID) error) (err error) {
	if storageErr := r.Storage(index).Iterate([]byte{}, func(blockIDBytes kvstore.Key, _ kvstore.Value) bool {
		blockID := new(models.BlockID)
		if _, err = blockID.FromBytes(blockIDBytes); err != nil {
			err = errors.Errorf("failed to parse blockID %s: %w", blockIDBytes, err)
		} else if err = processor(*blockID); err != nil {
			err = errors.Errorf("failed to process root block %s: %w", blockID, err)
		}

		return err == nil
	}); storageErr != nil {
		return errors.Errorf("failed to iterate over rootblocks: %w", storageErr)
	}

	return err
}
