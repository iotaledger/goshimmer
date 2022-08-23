package blockdag

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region BlockDAG ///////////////////////////////////////////////////////////////////////////////////////////////////////

// BlockDAG is a causally ordered DAG that forms the central data structure of the IOTA protocol.
type BlockDAG struct {
	// Events contains the Events of the BlockDAG.
	Events *Events

	// memStorage contains the in-memory storage of the BlockDAG.
	memStorage *memstorage.EpochStorage[models.BlockID, *Block]

	// solidifier contains the solidifier instance used to determine the solidity of Blocks.
	solidifier *causalorder.CausalOrder[models.BlockID, *Block]

	// evictionManager contains the manager used to orchestrate the eviction of old Blocks.
	evictionManager *eviction.LockableManager[models.BlockID]

	// orphanageMutex is a mutex that is used to synchronize updates to the orphanage flags.
	orphanageMutex *syncutils.DAGMutex[models.BlockID]
}

// New is the constructor for the BlockDAG and creates a new BlockDAG instance.
func New(evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[BlockDAG]) (newBlockDAG *BlockDAG) {
	newBlockDAG = options.Apply(&BlockDAG{
		Events:          newEvents(),
		memStorage:      memstorage.NewEpochStorage[models.BlockID, *Block](),
		evictionManager: evictionManager.Lockable(),
		orphanageMutex:  syncutils.NewDAGMutex[models.BlockID](),
	}, opts)

	newBlockDAG.solidifier = causalorder.New(
		evictionManager,
		newBlockDAG.Block,
		(*Block).IsSolid,
		newBlockDAG.markSolid,
		newBlockDAG.markInvalid,
		causalorder.WithReferenceValidator[models.BlockID](checkReference),
	)

	newBlockDAG.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(newBlockDAG.evictEpoch))

	return newBlockDAG
}

// Attach is used to attach new Blocks to the BlockDAG. It is the main function of the BlockDAG that triggers Events.
func (b *BlockDAG) Attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	if block, wasAttached, err = b.attach(data); wasAttached {
		b.Events.BlockAttached.Trigger(block)

		b.solidifier.Queue(block)
	}

	return
}

// Block retrieves a Block with metadata from the in-memory storage of the BlockDAG.
func (b *BlockDAG) Block(id models.BlockID) (block *Block, exists bool) {
	b.evictionManager.RLock()
	defer b.evictionManager.RUnlock()

	return b.block(id)
}

// SetInvalid marks a Block as invalid and propagates the invalidity to its future cone.
func (b *BlockDAG) SetInvalid(block *Block) (wasUpdated bool) {
	if wasUpdated = block.setInvalid(); wasUpdated {
		b.Events.BlockInvalid.Trigger(block)

		b.walkFutureCone(block.Children(), func(currentBlock *Block) []*Block {
			if !currentBlock.setInvalid() {
				return nil
			}

			b.Events.BlockInvalid.Trigger(currentBlock)

			return currentBlock.Children()
		})
	}

	return
}

// SetOrphaned marks a Block as orphaned and propagates it to its future cone.
func (b *BlockDAG) SetOrphaned(block *Block, orphaned bool) (updated bool, statusChanged bool) {
	b.orphanageMutex.Lock(block.ID())
	defer b.orphanageMutex.Unlock(block.ID())

	if !orphaned && !block.OrphanedBlocksInPastCone().Empty() {
		panic("tried to unorphan a block that still has orphaned parents")
	}

	updateEvent, updateFunc := b.orphanageUpdaters(orphaned)

	if updated, statusChanged = block.setOrphaned(orphaned); !updated {
		return
	}

	if statusChanged {
		updateEvent.Trigger(block)
	}

	b.propagateOrphanageUpdate(block.Children(), models.NewBlockIDs(block.ID()), updateEvent, updateFunc)

	return
}

// evictEpoch is used to evictEpoch the BlockDAG of all Blocks that are too old.
func (b *BlockDAG) evictEpoch(epochIndex epoch.Index) {
	b.solidifier.EvictEpoch(epochIndex)

	b.evictionManager.Lock()
	defer b.evictionManager.Unlock()

	b.memStorage.EvictEpoch(epochIndex)
}

func (b *BlockDAG) markSolid(block *Block) (err error) {
	block.setSolid()

	b.inheritOrphanage(block)

	b.Events.BlockSolid.Trigger(block)

	return nil
}

func (b *BlockDAG) markInvalid(block *Block, reason error) {
	b.SetInvalid(block)
}

// attach tries to attach the given Block to the BlockDAG.
func (b *BlockDAG) attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	b.evictionManager.RLock()
	defer b.evictionManager.RUnlock()

	if block, wasAttached, err = b.canAttach(data); !wasAttached {
		return
	}

	if block, wasAttached = b.memStorage.Get(data.ID().EpochIndex, true).RetrieveOrCreate(data.ID(), func() *Block { return NewBlock(data) }); !wasAttached {
		if wasAttached = block.update(data); !wasAttached {
			return
		}

		b.Events.MissingBlockAttached.Trigger(block)
	}

	block.ForEachParent(func(parent models.Parent) {
		b.registerChild(block, parent)
	})

	return
}

// canAttach determines if the Block can be attached (does not exist and addresses a recent epoch).
func (b *BlockDAG) canAttach(data *models.Block) (block *Block, canAttach bool, err error) {
	if b.evictionManager.IsTooOld(data.ID()) {
		return nil, false, errors.Errorf("block data with %s is too old", data.ID())
	}

	storedBlock, storedBlockExists := b.block(data.ID())
	if storedBlockExists && !storedBlock.IsMissing() {
		return storedBlock, false, nil
	}

	return b.canAttachToParents(storedBlock, data)
}

// canAttachToParents determines if the Block references parents in a non-pruned epoch. If a Block is found to violate
// this condition but exists as a missing entry, we mark it as invalid.
func (b *BlockDAG) canAttachToParents(storedBlock *Block, data *models.Block) (block *Block, canAttach bool, err error) {
	for _, parentID := range data.Parents() {
		if b.evictionManager.IsTooOld(parentID) {
			if storedBlock != nil {
				b.SetInvalid(storedBlock)
			}

			return storedBlock, false, errors.Errorf("parent %s of block %s is too old", parentID, data.ID())
		}
	}

	return storedBlock, true, nil
}

// registerChild registers the given Block as a child of the parent. It triggers a BlockMissing event if the referenced
// Block does not exist, yet.
func (b *BlockDAG) registerChild(child *Block, parent models.Parent) {
	if b.evictionManager.IsRootBlock(parent.ID) {
		return
	}

	parentBlock, _ := b.memStorage.Get(parent.ID.EpochIndex, true).RetrieveOrCreate(parent.ID, func() (newBlock *Block) {
		newBlock = NewBlock(models.NewEmptyBlock(parent.ID), WithMissing(true))

		b.Events.BlockMissing.Trigger(newBlock)

		return
	})

	parentBlock.appendChild(child, parent.Type)
}

// block retrieves the Block with given id from the mem-storage.
func (b *BlockDAG) block(id models.BlockID) (block *Block, exists bool) {
	if b.evictionManager.IsRootBlock(id) {
		return NewBlock(models.NewEmptyBlock(id), WithSolid(true)), true
	}

	storage := b.memStorage.Get(id.EpochIndex, false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

// walkFutureCone traverses the future cone of the given Block and calls the given callback for each Block.
func (b *BlockDAG) walkFutureCone(blocks []*Block, callback func(currentBlock *Block) (nextChildren []*Block)) {
	for childWalker := walker.New[*Block](false).PushAll(blocks...); childWalker.HasNext(); {
		childWalker.PushAll(callback(childWalker.Next())...)
	}
}

// inheritOrphanage inherits the orphanage state of the parents to the given Block.
func (b *BlockDAG) inheritOrphanage(block *Block) {
	b.orphanageMutex.RLock(block.Parents()...)
	defer b.orphanageMutex.RUnlock(block.Parents()...)

	if orphanedBlocks := b.orphanedBlocksInPastCone(block); !orphanedBlocks.Empty() {
		b.propagateOrphanageUpdate([]*Block{block}, orphanedBlocks, b.Events.BlockOrphaned, (*Block).addOrphanedBlocksInPastCone)
	}
}

// orphanedBlocksInPastCone returns an aggregation of the orphaned Blocks of the parents of the given Block.
func (b *BlockDAG) orphanedBlocksInPastCone(block *Block) (orphanedBlocks models.BlockIDs) {
	orphanedBlocks = models.NewBlockIDs()
	block.ForEachParent(func(parent models.Parent) {
		parentBlock, exists := b.Block(parent.ID)
		if !exists {
			panic(fmt.Sprintf("failed to find parent block with %s", parent.ID))
		}

		orphanedBlocks.AddAll(parentBlock.OrphanedBlocksInPastCone())
		if parentBlock.isOrphaned() {
			orphanedBlocks.Add(parentBlock.ID())
		}
	})

	return
}

// orphanageUpdaters returns the Event and update function used for handling the different types of orphanage updates.
func (b *BlockDAG) orphanageUpdaters(orphaned bool) (updateEvent *event.Event[*Block], updateFunc func(*Block, models.BlockIDs) (bool, bool)) {
	if !orphaned {
		return b.Events.BlockUnorphaned, (*Block).removeOrphanedBlocksInPastCone
	}

	return b.Events.BlockOrphaned, (*Block).addOrphanedBlocksInPastCone
}

// propagateOrphanageUpdate propagates the orphanage status of a Block to its future cone.
func (b *BlockDAG) propagateOrphanageUpdate(blocks []*Block, orphanedBlocks models.BlockIDs, updateEvent *event.Event[*Block], updateFunc func(*Block, models.BlockIDs) (bool, bool)) {
	b.walkFutureCone(blocks, func(currentBlock *Block) []*Block {
		b.orphanageMutex.Lock(currentBlock.ID())
		defer b.orphanageMutex.Unlock(currentBlock.ID())

		updated, statusChanged := updateFunc(currentBlock, orphanedBlocks)
		if !updated {
			return nil
		}

		if statusChanged {
			updateEvent.Trigger(currentBlock)
		}

		return currentBlock.Children()
	})
}

// checkReference checks if the reference between the child and its parent is valid.
func checkReference(child *Block, parent *Block) (err error) {
	if parent.invalid {
		return errors.Errorf("parent %s of child %s is marked as invalid", parent.ID(), child.ID())
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
