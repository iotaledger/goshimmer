package tangle

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

type Tangle struct {
	Events          Events
	metadataStorage *memstorage.EpochStorage[BlockID, *BlockMetadata]
	dbManager       *database.Manager
}

func (t *Tangle) AttachBlock(block *Block) {
	if /* abort if too old */ false {
		return
	}

	blockMetadata, isNew := t.publishNewBlock(block)
	if !isNew {
		return
	}

	if err := t.dbManager.Get(block.ID().EpochIndex, []byte{tangleold.PrefixBlock}).Set(block.IDBytes(), lo.PanicOnErr(block.Bytes())); err != nil {
		t.Events.Error.Trigger(errors.Errorf("failed to store block with %s on disk: %w", block.ID(), err))
		return
	}

	t.solidify(blockMetadata)
}

func (t *Tangle) BlockMetadata(blockID BlockID) (metadata *BlockMetadata, exists bool) {
	if t.isGenesisBlock(blockID) {
		return GenesisMetadata, true
	}

	return t.epochStorage(blockID).Get(blockID)
}

func (t *Tangle) publishNewBlock(block *Block) (blockMetadata *BlockMetadata, published bool) {
	blockMetadata, published = t.epochStorage(block.ID()).RetrieveOrCreate(block.ID(), t.fullMetadataFromBlock(block))

	if !published && !t.updateMissingMetadata(blockMetadata, block) {
		return blockMetadata, false
	}

	t.registerAsChild(block, blockMetadata)

	return blockMetadata, true
}

func (t *Tangle) updateMissingMetadata(blockMetadata *BlockMetadata, block *Block) (updated bool) {
	blockMetadata.Transaction(func() {
		if updated = blockMetadata.missing; updated {
			blockMetadata.missing = false
			blockMetadata.strongParents = block.ParentsByType(StrongParentType)
			blockMetadata.weakParents = block.ParentsByType(WeakParentType)
			blockMetadata.likedInsteadParents = block.ParentsByType(ShallowLikeParentType)

			t.Events.MissingBlockStored.Trigger(&MissingBlockStoredEvent{BlockID: blockMetadata.id})
		}
	})

	return updated
}

func (t *Tangle) isGenesisBlock(blockID BlockID) (isGenesisBlock bool) {
	return blockID == EmptyBlockID
}

func (t *Tangle) solidify(blockMetadata *BlockMetadata) {
	blockMetadata.Transaction(func() {
		if blockMetadata.missingParents = t.missingParents(blockMetadata); blockMetadata.missingParents == 0 {
			blockMetadata.solid = true

			t.Events.BlockSolid.Trigger(&BlockSolidEvent{BlockMetadata: blockMetadata})

			// TODO: Traverse children, decrease counter and trigger solid as well?
		}
	})
}

func (t *Tangle) missingParents(blockMetadata *BlockMetadata) (missingParents uint8) {
	for parentID := range blockMetadata.ParentIDs() {
		parentMetadata, _ := t.BlockMetadata(parentID)

		parentMetadata.Transaction(func() {
			if parentMetadata.missing {
				missingParents++
			}
		})
	}

	return missingParents
}

func (t *Tangle) epochStorage(blockID BlockID) (epochStorage *memstorage.Storage[BlockID, *BlockMetadata]) {
	return t.metadataStorage.Get(blockID.EpochIndex, true)
}

func (t *Tangle) registerAsChild(block *Block, metadata *BlockMetadata) {
	for parentID := range block.ParentsByType(StrongParentType) {
		if t.isGenesisBlock(parentID) {
			continue
		}

		parentMetadata, _ := t.epochStorage(parentID).RetrieveOrCreate(parentID, func() *BlockMetadata {
			t.Events.BlockMissing.Trigger(&BlockMissingEvent{BlockID: parentID})

			return &BlockMetadata{
				id:                   parentID,
				missing:              true,
				strongChildren:       make([]*BlockMetadata, 0),
				weakChildren:         make([]*BlockMetadata, 0),
				likedInsteadChildren: make([]*BlockMetadata, 0),
				transactionMutex:     syncutils.NewStarvingMutex(),
			}
		})
		parentMetadata.Transaction(func() {
			parentMetadata.strongChildren = append(parentMetadata.strongChildren, metadata)
		})
	}

	for parentID := range block.ParentsByType(WeakParentType) {
		if t.isGenesisBlock(parentID) {
			continue
		}

		parentMetadata, _ := t.epochStorage(parentID).RetrieveOrCreate(parentID, func() *BlockMetadata {
			t.Events.BlockMissing.Trigger(&BlockMissingEvent{BlockID: parentID})

			return &BlockMetadata{
				id:                   parentID,
				missing:              true,
				strongChildren:       make([]*BlockMetadata, 0),
				weakChildren:         make([]*BlockMetadata, 0),
				likedInsteadChildren: make([]*BlockMetadata, 0),
				transactionMutex:     syncutils.NewStarvingMutex(),
			}
		})
		parentMetadata.Transaction(func() {
			parentMetadata.weakChildren = append(parentMetadata.weakChildren, metadata)
		})
	}

	for parentID := range block.ParentsByType(ShallowLikeParentType) {
		if t.isGenesisBlock(parentID) {
			continue
		}

		parentMetadata, _ := t.epochStorage(parentID).RetrieveOrCreate(parentID, func() *BlockMetadata {
			t.Events.BlockMissing.Trigger(&BlockMissingEvent{BlockID: parentID})
			return &BlockMetadata{
				id:                   parentID,
				missing:              true,
				strongChildren:       make([]*BlockMetadata, 0),
				weakChildren:         make([]*BlockMetadata, 0),
				likedInsteadChildren: make([]*BlockMetadata, 0),
				transactionMutex:     syncutils.NewStarvingMutex(),
			}
		})
		parentMetadata.Transaction(func() {
			parentMetadata.likedInsteadChildren = append(parentMetadata.likedInsteadChildren, metadata)
		})
	}
}

func (t *Tangle) fullMetadataFromBlock(block *Block) func() *BlockMetadata {
	return func() *BlockMetadata {
		return &BlockMetadata{
			id:                   block.ID(),
			strongParents:        block.ParentsByType(StrongParentType),
			weakParents:          block.ParentsByType(WeakParentType),
			likedInsteadParents:  block.ParentsByType(ShallowLikeParentType),
			strongChildren:       make([]*BlockMetadata, 0),
			weakChildren:         make([]*BlockMetadata, 0),
			likedInsteadChildren: make([]*BlockMetadata, 0),
			transactionMutex:     syncutils.NewStarvingMutex(),
		}
	}
}
