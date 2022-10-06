package chainstorage

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type BlockStorage struct {
	chainStorage *ChainStorage
}

func (b *BlockStorage) Store(block *models.Block) {
	if err := b.chainStorage.bucketedStorage(block.ID().Index(), BlockStorageType).Set(lo.PanicOnErr(block.ID().Bytes()), lo.PanicOnErr(block.Bytes())); err != nil {
		b.chainStorage.Events.Error.Trigger(errors.Errorf("failed to store block %s: %w", block.ID, err))
	}
}

func (b *BlockStorage) Get(blockID models.BlockID) (block *models.Block, err error) {
	blockBytes, err := b.chainStorage.bucketedStorage(blockID.Index(), BlockStorageType).Get(lo.PanicOnErr(blockID.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil
		}

		return nil, errors.Errorf("failed to get block %s: %w", blockID, err)
	}

	block = new(models.Block)
	if _, err = block.FromBytes(blockBytes); err != nil {
		return nil, errors.Errorf("failed to parse block %s: %w", blockID, err)
	}
	block.SetID(blockID)

	return
}

func (b *BlockStorage) Delete(blockID models.BlockID) {
	if err := b.chainStorage.bucketedStorage(blockID.Index(), BlockStorageType).Delete(lo.PanicOnErr(blockID.Bytes())); err != nil {
		b.chainStorage.Events.Error.Trigger(errors.Errorf("failed to delete block %s: %w", blockID, err))
	}
}

func (b *BlockStorage) Storage(index epoch.Index) (storage kvstore.KVStore) {
	return b.chainStorage.bucketedStorage(index, BlockStorageType)
}
