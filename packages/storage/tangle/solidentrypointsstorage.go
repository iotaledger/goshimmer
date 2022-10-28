package tangle

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type SolidEntryPointsStorage struct {
	Storage func(index epoch.Index) kvstore.KVStore
}

func (s *SolidEntryPointsStorage) Store(block *models.Block) (err error) {
	if err = s.Storage(block.ID().Index()).Set(lo.PanicOnErr(block.ID().Bytes()), lo.PanicOnErr(block.Bytes())); err != nil {
		return errors.Errorf("failed to store solid entry point block %s: %w", block.ID, err)
	}

	return nil
}

func (s *SolidEntryPointsStorage) Get(blockID models.BlockID) (block *models.Block, err error) {
	blockBytes, err := s.Storage(blockID.Index()).Get(lo.PanicOnErr(blockID.Bytes()))
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, nil
		}

		return nil, errors.Errorf("failed to get solid entry point block %s: %w", blockID, err)
	}

	block = new(models.Block)
	if _, err = block.FromBytes(blockBytes); err != nil {
		return nil, errors.Errorf("failed to parse solid entry point block %s: %w", blockID, err)
	}
	block.SetID(blockID)

	return
}

func (s *SolidEntryPointsStorage) GetAll(index epoch.Index) (rootBlocks *set.AdvancedSet[*models.Block]) {
	rootBlocks = set.NewAdvancedSet[*models.Block]()
	s.Stream(index, func(block *models.Block) {
		rootBlocks.Add(block)
	})
	return
}

func (s *SolidEntryPointsStorage) Delete(blockID models.BlockID) (err error) {
	if err = s.Storage(blockID.Index()).Delete(lo.PanicOnErr(blockID.Bytes())); err != nil {
		return errors.Errorf("failed to delete solid entry point block %s: %w", blockID, err)
	}

	return nil
}

func (s *SolidEntryPointsStorage) Stream(index epoch.Index, callback func(*models.Block)) {
	s.Storage(index).Iterate([]byte{}, func(blockIDBytes kvstore.Key, blockBytes kvstore.Value) bool {
		blockID := new(models.BlockID)
		blockID.FromBytes(blockIDBytes)
		block := new(models.Block)
		block.FromBytes(blockBytes)
		block.SetID(*blockID)
		callback(block)
		return true
	})
}
