package prunable

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Blocks struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

func NewBlocks(database *database.Manager, storagePrefix byte) (newBlocks *Blocks) {
	return &Blocks{
		Storage: lo.Bind([]byte{storagePrefix}, database.Get),
	}
}

func (b *Blocks) Load(id models.BlockID) (block *models.Block, err error) {
	blockBytes, err := b.Storage(id.Index()).Get(lo.PanicOnErr(id.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil
		}

		return nil, errors.Errorf("failed to get block %s: %w", id, err)
	}

	block = new(models.Block)
	if _, err = block.FromBytes(blockBytes); err != nil {
		return nil, errors.Errorf("failed to parse block %s: %w", id, err)
	}
	block.SetID(id)

	return
}

func (b *Blocks) Store(block *models.Block) (err error) {
	if err = b.Storage(block.ID().Index()).Set(lo.PanicOnErr(block.ID().Bytes()), lo.PanicOnErr(block.Bytes())); err != nil {
		return errors.Errorf("failed to store block %s: %w", block.ID, err)
	}

	return nil
}

func (b *Blocks) Delete(id models.BlockID) (err error) {
	if err = b.Storage(id.Index()).Delete(lo.PanicOnErr(id.Bytes())); err != nil {
		return errors.Errorf("failed to delete block %s: %w", id, err)
	}

	return nil
}
