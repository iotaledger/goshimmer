package prunable

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
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
func (r *RootBlocks) Store(id models.BlockID, commitmentID commitment.ID) (err error) {
	if err = r.Storage(id.Index()).Set(lo.PanicOnErr(id.Bytes()), lo.PanicOnErr(commitmentID.Bytes())); err != nil {
		return errors.Wrapf(err, "failed to store solid entry point block %s with commitment %s", id, commitmentID)
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

// Stream streams all root blocks for a slot index.
func (r *RootBlocks) Stream(index slot.Index, processor func(id models.BlockID, commitmentID commitment.ID) error) (err error) {
	if storageErr := r.Storage(index).Iterate([]byte{}, func(blockIDBytes kvstore.Key, commitmentIDBytes kvstore.Value) bool {
		id := new(models.BlockID)
		commitmentID := new(commitment.ID)
		if _, err = id.FromBytes(blockIDBytes); err != nil {
			err = errors.Wrapf(err, "failed to parse blockID %s", blockIDBytes)
		} else if _, err = commitmentID.FromBytes(commitmentIDBytes); err != nil {
			err = errors.Wrapf(err, "failed to parse commitmentID %s", commitmentIDBytes)
		} else if err = processor(*id, *commitmentID); err != nil {
			err = errors.Wrapf(err, "failed to process root block %s with commitment %s", *id, *commitmentID)
		}

		return err == nil
	}); storageErr != nil {
		return errors.Wrapf(storageErr, "failed to iterate over rootblocks")
	}

	return err
}
