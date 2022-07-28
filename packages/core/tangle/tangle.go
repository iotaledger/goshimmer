package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/options"
	"github.com/iotaledger/hive.go/generics/walker"

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
	Events *Events

	dbManager         *database.Manager
	dbManagerPath     string
	memStorage        *memstorage.EpochStorage[models.BlockID, *Block]
	maxDroppedEpoch   epoch.Index
	solidifier        *causalorder.CausalOrder[models.BlockID, *Block]
	isSolidEntryPoint func(models.BlockID) bool

	sync.RWMutex
}

// New is the constructor for the Tangle.
func New(opts ...options.Option[Tangle]) (newTangle *Tangle) {
	return options.Apply(&Tangle{
		Events:            newEvents(),
		memStorage:        memstorage.NewEpochStorage[models.BlockID, *Block](),
		dbManagerPath:     "/tmp/",
		isSolidEntryPoint: IsGenesisBlock,
	}, opts).initDBManager().initSolidifier()
}

// Attach is used to attach new Blocks to the Tangle. This function also triggers the necessary events.
func (t *Tangle) Attach(block *models.Block) (blockMetadata *Block, isNew bool) {
	t.RLock()
	defer t.RUnlock()

	if t.isTooOld(block) {
		// TODO: propagate invalidity to any existing future-cone of the block
		return
	}

	if blockMetadata, isNew = t.publishBlockData(block); !isNew {
		return
	}

	t.Events.BlockStored.Trigger(blockMetadata)

	t.solidifier.Queue(blockMetadata)

	return
}

func (t *Tangle) Prune(epochIndex epoch.Index) {
	t.Lock()
	defer t.Unlock()

	t.solidifier.Prune(epochIndex)

	for i := t.maxDroppedEpoch + 1; i <= epochIndex; i++ {
		t.memStorage.Drop(i)
	}

	t.maxDroppedEpoch = epochIndex
}

// Block retrieves a Block with metadata from the cache.
func (t *Tangle) Block(blockID models.BlockID) (metadata *Block, exists bool) {
	t.RLock()
	defer t.RUnlock()

	return t.block(blockID)
}

// SetInvalid is used to mark a Block as invalid and propagate invalidity to its future cone. Locks the metadata mutex.
func (t *Tangle) SetInvalid(metadata *Block) (updated bool) {
	if updated = metadata.setInvalid(); !updated {
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
	t.dbManager = database.NewManager(t.dbManagerPath)

	return t
}

func (t *Tangle) initSolidifier() (self *Tangle) {
	t.solidifier = causalorder.New(
		t.Block,
		(*Block).isSolid,
		(*Block).setSolid,
		t.isSolidEntryPoint,
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

func (t *Tangle) block(blockID models.BlockID) (metadata *Block, exists bool) {
	if t.isSolidEntryPoint(blockID) {
		return SolidEntrypointMetadata(blockID), true
	}

	if t.isBlockIDTooOld(blockID) {
		return nil, false
	}

	return t.epochStorage(blockID).Get(blockID)
}

func (t *Tangle) publishBlockData(data *models.Block) (block *Block, published bool) {
	block, published = t.epochStorage(data.ID()).RetrieveOrCreate(data.ID(), func() *Block {
		return NewBlock(data)
	})

	if !published && !t.publishMissingData(block, data) {
		return block, false
	}

	t.registerAsChild(block)
	t.storeBlock(data)

	return block, true
}

func (t *Tangle) storeBlock(block *models.Block) {
	if err := t.dbManager.Get(block.ID().EpochIndex, []byte{tangleold.PrefixBlock}).Set(block.IDBytes(), lo.PanicOnErr(block.Bytes())); err != nil {
		t.Events.Error.Trigger(errors.Errorf("failed to store block with %s on disk: %w", block.ID(), err))
	}
}

func (t *Tangle) publishMissingData(block *Block, model *models.Block) (wasPublished bool) {
	block.Lock()
	defer block.Unlock()

	if wasPublished = block.publishMissingBlock(model); wasPublished {
		t.Events.MissingBlockStored.Trigger(block)
	}

	return
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
		if t.isSolidEntryPoint(blockID) {
			continue
		}

		parentMetadata, _ := t.epochStorage(blockID).RetrieveOrCreate(blockID, func() (newBlock *Block) {
			newBlock = NewBlock(models.NewEmptyBlock(blockID), WithMissing(true))

			t.Events.BlockMissing.Trigger(newBlock)

			return
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

		if !child.setInvalid() {
			continue
		}

		t.Events.BlockInvalid.Trigger(child)

		propagationWalker.PushAll(child.Children()...)
	}
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
	return !t.isSolidEntryPoint(blockID) && blockID.EpochIndex <= t.maxDroppedEpoch
}

func (t *Tangle) epochStorage(blockID models.BlockID) (epochStorage *memstorage.Storage[models.BlockID, *Block]) {
	return t.memStorage.Get(blockID.EpochIndex, true)
}
