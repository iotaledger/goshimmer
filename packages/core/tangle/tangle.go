package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

type Tangle struct {
	Events            *Events
	memStorage        *memstorage.EpochStorage[BlockID, *BlockMetadata]
	dbManager         *database.Manager
	dbManagerPath     string
	isSolidEntryPoint func(BlockID) bool
	maxDroppedEpoch   epoch.Index
	pruningMutex      sync.RWMutex
	solidifier        *CausalOrderer[BlockID, *BlockMetadata]
}

func NewTangle(opts ...options.Option[Tangle]) (newTangle *Tangle) {
	newTangle = options.Apply(&Tangle{
		Events:        newEvents(),
		memStorage:    memstorage.NewEpochStorage[BlockID, *BlockMetadata](),
		dbManagerPath: "/tmp/",
		isSolidEntryPoint: func(id BlockID) bool {
			return id == EmptyBlockID
		},
	}, opts)
	newTangle.dbManager = database.NewManager(newTangle.dbManagerPath)
	newTangle.solidifier = NewCausalOrderer(newTangle.BlockMetadata, (*BlockMetadata).isSolid, (*BlockMetadata).setSolid)

	newTangle.solidifier.ElementOrdered.Hook(event.NewClosure(newTangle.Events.BlockSolid.Trigger))
	newTangle.solidifier.ElementInvalid.Hook(event.NewClosure(func(blockMetadata *BlockMetadata) {
		newTangle.SetInvalid(blockMetadata)
	}))

	return newTangle
}

func (t *Tangle) AttachBlock(block *Block) {
	t.pruningMutex.RLock()
	defer t.pruningMutex.RUnlock()

	if t.isTooOld(block) {
		// TODO: propagate invalidity to any existing futurecone of the block
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

	t.Events.BlockStored.Trigger(blockMetadata)

	t.solidifier.Queue(blockMetadata)
}

func (t *Tangle) BlockMetadata(blockID BlockID) (metadata *BlockMetadata, exists bool) {
	t.pruningMutex.RLock()
	defer t.pruningMutex.RUnlock()
	return t.blockMetadata(blockID)
}

func (t *Tangle) blockMetadata(blockID BlockID) (metadata *BlockMetadata, exists bool) {
	if t.isBlockIDTooOld(blockID) {
		return nil, false
	}

	if t.IsGenesisBlock(blockID) {
		return GenesisMetadata, true
	}

	return t.epochStorage(blockID).Get(blockID)
}

func (t *Tangle) SetInvalid(metadata *BlockMetadata) (updated bool) {
	if updated = metadata.setInvalid(); updated {
		t.Events.BlockInvalid.Trigger(metadata)

		t.propagateInvalidityToChildren(metadata)
	}

	return
}

func (t *Tangle) propagateInvalidityToChildren(entity *BlockMetadata) {
	for propagationWalker := walker.New[*BlockMetadata](true).Push(entity); propagationWalker.HasNext(); {
		for _, childMetadata := range propagationWalker.Next().Children() {
			if childMetadata.setInvalid() {
				t.Events.BlockInvalid.Trigger(childMetadata)

				propagationWalker.Push(childMetadata)
			}
		}
	}
}

func (t *Tangle) IsGenesisBlock(blockID BlockID) (isGenesisBlock bool) {
	return blockID == EmptyBlockID
}

func (t *Tangle) Shutdown() {
	t.dbManager.Shutdown()
}

func (t *Tangle) publishNewBlock(block *Block) (blockMetadata *BlockMetadata, published bool) {
	blockMetadata, published = t.epochStorage(block.ID()).RetrieveOrCreate(block.ID(), fullMetadataFromBlock(block))

	if !published && !t.updateMissingMetadata(blockMetadata, block) {
		return blockMetadata, false
	}

	t.registerAsChild(blockMetadata)

	return blockMetadata, true
}

func (t *Tangle) updateMissingMetadata(blockMetadata *BlockMetadata, block *Block) (updated bool) {
	blockMetadata.Lock()
	defer blockMetadata.Unlock()

	if updated = blockMetadata.missing; updated {
		blockMetadata.missing = false
		blockMetadata.strongParents = block.ParentsByType(StrongParentType)
		blockMetadata.weakParents = block.ParentsByType(WeakParentType)
		blockMetadata.likedInsteadParents = block.ParentsByType(ShallowLikeParentType)

		t.Events.MissingBlockStored.Trigger(blockMetadata)
	}

	return updated
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

func (t *Tangle) updateParentsMetadata(blockIDs BlockIDs, updateParentsFunc func(metadata *BlockMetadata)) {
	for blockID := range blockIDs {
		if t.IsGenesisBlock(blockID) {
			continue
		}

		parentMetadata, _ := t.epochStorage(blockID).RetrieveOrCreate(blockID, func() *BlockMetadata {
			missingBlockMetadata := &BlockMetadata{
				id:                   blockID,
				missing:              true,
				strongChildren:       make([]*BlockMetadata, 0),
				weakChildren:         make([]*BlockMetadata, 0),
				likedInsteadChildren: make([]*BlockMetadata, 0),
				StarvingMutex:        syncutils.NewStarvingMutex(),
			}

			t.Events.BlockMissing.Trigger(missingBlockMetadata)

			return missingBlockMetadata
		})

		parentMetadata.Lock()
		updateParentsFunc(parentMetadata)
		parentMetadata.Unlock()
	}
}

func (t *Tangle) isTooOld(block *Block) (isTooOld bool) {
	if t.isBlockIDTooOld(block.ID()) {
		return true
	}

	for _, parentID := range block.Parents() {
		if t.isBlockIDTooOld(parentID) {
			return true
		}
	}

	return false
}

func (t *Tangle) isBlockIDTooOld(blockID BlockID) bool {
	return !t.IsGenesisBlock(blockID) && blockID.EpochIndex <= t.maxDroppedEpoch
}

func (t *Tangle) epochStorage(blockID BlockID) (epochStorage *memstorage.Storage[BlockID, *BlockMetadata]) {
	return t.memStorage.Get(blockID.EpochIndex, true)
}
