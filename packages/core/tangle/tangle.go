package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is the central data structure of the IOTA protocol.
type Tangle struct {
	Events          *Events
	memStorage      *memstorage.EpochStorage[models.BlockID, *Block]
	dbManager       *database.Manager
	maxDroppedEpoch epoch.Index
	pruningMutex    sync.RWMutex
	solidifier      *causalorder.CausalOrder[models.BlockID, *Block]

	optsDBManagerPath     string
	optsIsSolidEntryPoint func(models.BlockID) bool
}

// New is the constructor for the Tangle.
func New(opts ...options.Option[Tangle]) (newTangle *Tangle) {
	return options.Apply(&Tangle{
		Events:                newEvents(),
		memStorage:            memstorage.NewEpochStorage[models.BlockID, *Block](),
		optsDBManagerPath:     "/tmp/",
		optsIsSolidEntryPoint: IsGenesisBlock,
	}, opts).initDBManager().initSolidifier()
}

// AttachBlock is used to attach new Blocks to the Tangle. This function also triggers the necessary events.
func (t *Tangle) AttachBlock(block *models.Block) {
	t.pruningMutex.RLock()
	defer t.pruningMutex.RUnlock()

	if t.isTooOld(block) {
		// TODO: propagate invalidity to any existing future-cone of the block
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

func (t *Tangle) DropEpoch(epochIndex epoch.Index) {
	t.pruningMutex.Lock()
	defer t.pruningMutex.Unlock()

	t.solidifier.Prune(epochIndex)

	for i := t.maxDroppedEpoch + 1; i <= epochIndex; i++ {
		t.memStorage.Drop(i)
	}

	t.maxDroppedEpoch = epochIndex
}

// BlockMetadata retrieves the BlockMetadata with the given BlockID.
func (t *Tangle) BlockMetadata(blockID models.BlockID) (metadata *Block, exists bool) {
	t.pruningMutex.RLock()
	defer t.pruningMutex.RUnlock()

	return t.blockMetadata(blockID)
}

// SetInvalid is used to mark a Block as invalid and propagate invalidity to its future cone. Locks the metadata mutex.
func (t *Tangle) SetInvalid(metadata *Block) (updated bool) {
	if updated = t.setBlockInvalid(metadata); !updated {
		return
	}

	t.Events.BlockInvalid.Trigger(metadata)

	t.propagateInvalidityToChildren(metadata.Children())

	return
}

// Shutdown marks the tangle as stopped, so it will not accept any new blocks (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.dbManager.Shutdown()
}

func (t *Tangle) initDBManager() (self *Tangle) {
	t.dbManager = database.NewManager(t.optsDBManagerPath)

	return t
}

func (t *Tangle) initSolidifier() (self *Tangle) {
	t.solidifier = causalorder.New(
		t.BlockMetadata,
		(*Block).isSolid,
		(*Block).setSolid,
		t.optsIsSolidEntryPoint,
		causalorder.WithReferenceValidator[models.BlockID](func(entity *Block, parent *Block) bool {
			return !parent.invalid
		}),
	)

	t.solidifier.Events.Emit.Hook(event.NewClosure(t.Events.BlockSolid.Trigger))
	t.solidifier.Events.Drop.Attach(event.NewClosure(func(blockMetadata *Block) {
		t.SetInvalid(blockMetadata)
	}))

	return t
}

func (t *Tangle) blockMetadata(blockID models.BlockID) (metadata *Block, exists bool) {
	if t.optsIsSolidEntryPoint(blockID) {
		return SolidEntrypointMetadata(blockID), true
	}

	if t.isBlockIDTooOld(blockID) {
		return nil, false
	}

	return t.epochStorage(blockID).Get(blockID)
}

func (t *Tangle) publishNewBlock(block *models.Block) (blockMetadata *Block, published bool) {
	blockMetadata, published = t.epochStorage(block.ID()).RetrieveOrCreate(block.ID(), func() *Block {
		return &Block{
			Block:                block,
			strongChildren:       make([]*Block, 0),
			weakChildren:         make([]*Block, 0),
			likedInsteadChildren: make([]*Block, 0),
			StarvingMutex:        syncutils.NewStarvingMutex(),
		}
	})

	if !published && !t.updateMissingMetadata(blockMetadata, block) {
		return blockMetadata, false
	}

	t.registerAsChild(blockMetadata)

	return blockMetadata, true
}

func (t *Tangle) updateMissingMetadata(blockMetadata *Block, block *models.Block) (updated bool) {
	blockMetadata.Lock()
	defer blockMetadata.Unlock()

	if updated = blockMetadata.missing; updated {
		blockMetadata.Block.Update(block)
		blockMetadata.missing = false

		t.Events.MissingBlockStored.Trigger(blockMetadata)
	}

	return updated
}

func (t *Tangle) registerAsChild(metadata *Block) {
	t.updateParentsMetadata(metadata.ParentsByType(models.StrongParentType), func(parentMetadata *Block) {
		parentMetadata.strongChildren = append(parentMetadata.strongChildren, metadata)
	})

	t.updateParentsMetadata(metadata.ParentsByType(models.WeakParentType), func(parentMetadata *Block) {
		parentMetadata.weakChildren = append(parentMetadata.weakChildren, metadata)
	})

	t.updateParentsMetadata(metadata.ParentsByType(models.ShallowLikeParentType), func(parentMetadata *Block) {
		parentMetadata.likedInsteadChildren = append(parentMetadata.likedInsteadChildren, metadata)
	})
}

func (t *Tangle) updateParentsMetadata(blockIDs models.BlockIDs, updateParentsFunc func(metadata *Block)) {
	for blockID := range blockIDs {
		if t.optsIsSolidEntryPoint(blockID) {
			continue
		}

		parentMetadata, _ := t.epochStorage(blockID).RetrieveOrCreate(blockID, func() *Block {
			missingBlockMetadata := &Block{
				Block:                models.NewEmptyBlock(blockID),
				missing:              true,
				strongChildren:       make([]*Block, 0),
				weakChildren:         make([]*Block, 0),
				likedInsteadChildren: make([]*Block, 0),
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

func (t *Tangle) propagateInvalidityToChildren(children []*Block) {
	propagationWalker := walker.New[*Block](true).PushAll(children...)
	for propagationWalker.HasNext() {
		child := propagationWalker.Next()

		if !t.setBlockInvalid(child) {
			continue
		}

		t.Events.BlockInvalid.Trigger(child)

		propagationWalker.PushAll(child.Children()...)
	}
}

func (t *Tangle) setBlockInvalid(block *Block) (propagated bool) {
	block.Lock()
	defer block.Unlock()

	return block.setInvalid()
}

func (t *Tangle) isTooOld(block *models.Block) (isTooOld bool) {
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

func (t *Tangle) isBlockIDTooOld(blockID models.BlockID) bool {
	return !t.optsIsSolidEntryPoint(blockID) && blockID.EpochIndex <= t.maxDroppedEpoch
}

func (t *Tangle) epochStorage(blockID models.BlockID) (epochStorage *memstorage.Storage[models.BlockID, *Block]) {
	return t.memStorage.Get(blockID.EpochIndex, true)
}
