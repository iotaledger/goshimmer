package prunable

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/slot"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
)

type RootBlocks struct {
	Storage func(index slot.Index) kvstore.KVStore
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
		return errors.Wrapf(err, "failed to store solid entry point block %s", id)
	}

	return nil
}

// Has returns true if the given blockID is a root block.
func (r *RootBlocks) Has(blockID models.BlockID) (has bool, err error) {
	has, err = r.Storage(blockID.Index()).Has(lo.PanicOnErr(blockID.Bytes()))
	if err != nil {
		return false, errors.Wrapf(err, "failed to delete solid entry point block %s", blockID)
	}

	return has, nil
}

// Delete deletes the given blockID from the root blocks.
func (r *RootBlocks) Delete(blockID models.BlockID) (err error) {
	if err = r.Storage(blockID.Index()).Delete(lo.PanicOnErr(blockID.Bytes())); err != nil {
		return errors.Wrapf(err, "failed to delete solid entry point block %s", blockID)
	}

	return nil
}

// LoadAll loads all root blocks for an slot index.
func (r *RootBlocks) LoadAll(index slot.Index) (solidEntryPoints *advancedset.AdvancedSet[models.BlockID]) {
	solidEntryPoints = advancedset.NewAdvancedSet[models.BlockID]()
	if err := r.Stream(index, func(id models.BlockID) error {
		solidEntryPoints.Add(id)
		return nil
	}); err != nil {
		panic(errors.Wrapf(err, "failed to load all rootblocks for slot %d", index))
	}
	return
}

// StoreAll stores all passed root blocks.
func (r *RootBlocks) StoreAll(rootBlocks *advancedset.AdvancedSet[models.BlockID]) (err error) {
	for it := rootBlocks.Iterator(); it.HasNext(); {
		if err := r.Store(it.Next()); err != nil {
			return errors.Wrap(err, "failed to store rootblocks")
		}
	}
	return nil
}

// Stream streams all root blocks for an slot index.
func (r *RootBlocks) Stream(index slot.Index, processor func(models.BlockID) error) (err error) {
	if storageErr := r.Storage(index).Iterate([]byte{}, func(blockIDBytes kvstore.Key, _ kvstore.Value) bool {
		blockID := new(models.BlockID)
		if _, err = blockID.FromBytes(blockIDBytes); err != nil {
			err = errors.Wrapf(err, "failed to parse blockID %s", blockIDBytes)
		} else if err = processor(*blockID); err != nil {
			err = errors.Wrapf(err, "failed to process root block %s", blockID)
		}

		return err == nil
	}); storageErr != nil {
		return errors.Wrapf(storageErr, "failed to iterate over rootblocks")
	}

	return err
}
