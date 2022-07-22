package tangle

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
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

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (t *Tangle) Setup() {
	t.Events.BlockSolid.Attach(event.NewClosure(func(event *BlockSolidEvent) {
		t.updateMissingParents(event.BlockMetadata)
	}))
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

	t.registerAsChild(blockMetadata)

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
		}
	})
}

func (t *Tangle) updateMissingParents(metadata *BlockMetadata) {
	t.processChildren(metadata.strongChildren)
	t.processChildren(metadata.weakChildren)
	t.processChildren(metadata.likedInsteadChildren)
}

func (t *Tangle) processChildren(childrenMetadata []*BlockMetadata) {
	for _, childMetadata := range childrenMetadata {
		childMetadata.Transaction(func() {
			childMetadata.missingParents--
			if childMetadata.missingParents == 0 {
				t.Events.BlockSolid.Trigger(&BlockSolidEvent{BlockMetadata: childMetadata})
			}
		})
	}
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

func (t *Tangle) registerAsChild(metadata *BlockMetadata) {
	t.updateParentsMetadata(metadata.strongParents, func(parentMetadata *BlockMetadata) {
		parentMetadata.strongChildren = append(parentMetadata.strongChildren, metadata)
	})

	t.updateParentsMetadata(metadata.weakParents, func(parentMetadata *BlockMetadata) {
		parentMetadata.weakChildren = append(parentMetadata.weakChildren, metadata)
	})

	t.updateParentsMetadata(metadata.likedInsteadParents, func(parentMetadata *BlockMetadata) {
		parentMetadata.likedInsteadChildren = append(parentMetadata.likedInsteadChildren, metadata)
	})
}

func (t *Tangle) updateParentsMetadata(blockIDs BlockIDs, updateFunc func(metadata *BlockMetadata)) {
	for blockID := range blockIDs {
		if t.isGenesisBlock(blockID) {
			continue
		}

		parentMetadata, _ := t.epochStorage(blockID).RetrieveOrCreate(blockID, func() *BlockMetadata {
			t.Events.BlockMissing.Trigger(&BlockMissingEvent{BlockID: blockID})
			return &BlockMetadata{
				id:                   blockID,
				missing:              true,
				strongChildren:       make([]*BlockMetadata, 0),
				weakChildren:         make([]*BlockMetadata, 0),
				likedInsteadChildren: make([]*BlockMetadata, 0),
				transactionMutex:     syncutils.NewStarvingMutex(),
			}
		})
		parentMetadata.Transaction(func() {
			updateFunc(parentMetadata)
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
