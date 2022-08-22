package tangle

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is a causally ordered DAG that forms the central data structure of the IOTA protocol.
type Tangle struct {
	// Events contains the Events of Tangle.
	Events *Events

	// memStorage contains the in-memory storage of the Tangle.
	memStorage *memstorage.EpochStorage[models.BlockID, *Block]

	// solidifier contains the solidifier instance used to determine the solidity of Blocks.
	solidifier *causalorder.CausalOrder[models.BlockID, *Block]

	// evictionManager contains the manager used to orchestrate the eviction of old Blocks.
	evictionManager *eviction.LockableManager[models.BlockID]
}

// New is the constructor for the Tangle and creates a new Tangle instance.
func New(evictionManager *eviction.Manager[models.BlockID], opts ...options.Option[Tangle]) (newTangle *Tangle) {
	newTangle = options.Apply(&Tangle{
		Events:          newEvents(),
		memStorage:      memstorage.NewEpochStorage[models.BlockID, *Block](),
		evictionManager: evictionManager.Lockable(),
	}, opts)

	newTangle.solidifier = causalorder.New(
		evictionManager,
		newTangle.Block,
		(*Block).IsSolid,
		newTangle.markSolid,
		newTangle.markInvalid,
		causalorder.WithReferenceValidator[models.BlockID](checkReference),
	)

	newTangle.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(newTangle.evictEpoch))

	return newTangle
}

// Attach is used to attach new Blocks to the Tangle. It is the main function of the Tangle that triggers Events.
func (t *Tangle) Attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	if block, wasAttached, err = t.attach(data); wasAttached {
		t.Events.BlockAttached.Trigger(block)

		t.solidifier.Queue(block)
	}

	return
}

// Block retrieves a Block with metadata from the in-memory storage of the Tangle.
func (t *Tangle) Block(id models.BlockID) (block *Block, exists bool) {
	t.evictionManager.RLock()
	defer t.evictionManager.RUnlock()

	return t.block(id)
}

// SetInvalid marks a Block as invalid and propagates the invalidity to its future cone.
func (t *Tangle) SetInvalid(block *Block) (wasUpdated bool) {
	if wasUpdated = block.setInvalid(); wasUpdated {
		t.Events.BlockInvalid.Trigger(block)

		t.walkFutureCone(block, t.propagateInvalidity)
	}

	return
}

// SetOrphaned marks a Block as orphaned and propagates it to its future cone.
func (t *Tangle) SetOrphaned(block *Block) (becameOrphaned bool) {
	updated, becameOrphaned := block.setOrphaned(true)
	if !updated {
		return false
	}

	if becameOrphaned {
		t.Events.BlockOrphaned.Trigger(block)
	}

	t.walkFutureCone(block, func(currentBlock *Block) (nextChildren []*Block) {
		return t.propagateOrphanage(block, currentBlock)
	})

	return becameOrphaned
}

// SetUnorphaned marks a Block as unorphaned and propagates it to its future cone.
func (t *Tangle) SetUnorphaned(block *Block) (becameUnorphaned bool) {
	if len(block.OrphanedBlocksInPastCone()) != 0 {
		panic("tried to unorphan a block that still has orphaned parents")
	}

	updated, becameUnorphaned := block.setOrphaned(false)
	if !updated {
		return
	}

	if becameUnorphaned {
		t.Events.BlockUnorphaned.Trigger(block)
	}

	t.walkFutureCone(block, func(currentBlock *Block) (nextChildren []*Block) {
		return t.propagateUnorphanage(block, currentBlock)
	})

	return becameUnorphaned
}

// evictEpoch is used to evictEpoch the Tangle of all Blocks that are too old.
func (t *Tangle) evictEpoch(epochIndex epoch.Index) {
	t.solidifier.EvictEpoch(epochIndex)

	t.evictionManager.Lock()
	defer t.evictionManager.Unlock()

	t.memStorage.EvictEpoch(epochIndex)
}

func (t *Tangle) markSolid(block *Block) (err error) {
	block.setSolid()

	t.Events.BlockSolid.Trigger(block)

	return nil
}

func (t *Tangle) markInvalid(block *Block, reason error) {
	t.SetInvalid(block)
}

// attach tries to attach the given Block to the Tangle.
func (t *Tangle) attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	t.evictionManager.RLock()
	defer t.evictionManager.RUnlock()

	if block, wasAttached, err = t.canAttach(data); !wasAttached {
		return
	}

	if block, wasAttached = t.memStorage.Get(data.ID().EpochIndex, true).RetrieveOrCreate(data.ID(), func() *Block { return NewBlock(data) }); !wasAttached {
		if wasAttached = block.update(data); !wasAttached {
			return
		}

		t.Events.MissingBlockAttached.Trigger(block)
	}

	block.ForEachParent(func(parent models.Parent) {
		t.registerChild(block, parent)
	})

	return
}

// canAttach determines if the Block can be attached (does not exist and addresses a recent epoch).
func (t *Tangle) canAttach(data *models.Block) (block *Block, canAttach bool, err error) {
	if t.evictionManager.IsTooOld(data.ID()) {
		return nil, false, errors.Errorf("block data with %s is too old", data.ID())
	}

	storedBlock, storedBlockExists := t.block(data.ID())
	if storedBlockExists && !storedBlock.IsMissing() {
		return storedBlock, false, nil
	}

	return t.canAttachToParents(storedBlock, data)
}

// canAttachToParents determines if the Block references parents in a non-pruned epoch. If a Block is found to violate
// this condition but exists as a missing entry, we mark it as invalid.
func (t *Tangle) canAttachToParents(storedBlock *Block, data *models.Block) (block *Block, canAttach bool, err error) {
	for _, parentID := range data.Parents() {
		if t.evictionManager.IsTooOld(parentID) {
			if storedBlock != nil {
				t.SetInvalid(storedBlock)
			}

			return storedBlock, false, errors.Errorf("parent %s of block %s is too old", parentID, data.ID())
		}
	}

	return storedBlock, true, nil
}

// registerChild registers the given Block as a child of the parent. It triggers a BlockMissing event if the referenced
// Block does not exist, yet.
func (t *Tangle) registerChild(child *Block, parent models.Parent) {
	if t.evictionManager.IsRootBlock(parent.ID) {
		return
	}

	parentBlock, _ := t.memStorage.Get(parent.ID.EpochIndex, true).RetrieveOrCreate(parent.ID, func() (newBlock *Block) {
		newBlock = NewBlock(models.NewEmptyBlock(parent.ID), WithMissing(true))

		t.Events.BlockMissing.Trigger(newBlock)

		return
	})

	parentBlock.appendChild(child, parent.Type)

	if _, becameOrphaned := child.addOrphanedBlocksInPastCone(parentBlock.OrphanedBlocksInPastCone()); becameOrphaned {
		t.Events.BlockOrphaned.Trigger(child)
	}
}

// block retrieves the Block with given id from the mem-storage.
func (t *Tangle) block(id models.BlockID) (block *Block, exists bool) {
	if t.evictionManager.IsRootBlock(id) {
		return NewBlock(models.NewEmptyBlock(id), WithSolid(true)), true
	}

	storage := t.memStorage.Get(id.EpochIndex, false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

// propagateInvalidity marks the children (and their future cone) as invalid.
func (t *Tangle) propagateInvalidity(block *Block) (nextChildren []*Block) {
	if !block.setInvalid() {
		return
	}

	t.Events.BlockInvalid.Trigger(block)

	return block.Children()
}

// propagateOrphanage propagates the orphanage of the given Block to its future cone.
func (t *Tangle) propagateOrphanage(block, currentBlock *Block) (nextChildren []*Block) {
	wasAdded, becameOrphaned := currentBlock.addOrphanedBlocksInPastCone(models.NewBlockIDs(block.ID()))
	if !wasAdded {
		return
	}

	if becameOrphaned {
		t.Events.BlockOrphaned.Trigger(currentBlock)
	}

	return currentBlock.Children()
}

func (t *Tangle) propagateUnorphanage(block, currentBlock *Block) (nextChildren []*Block) {
	wasRemoved, wasChildUnorphaned := currentBlock.removeOrphanedBlockInPastCone(block.ID())
	if !wasRemoved {
		return nil
	}

	if wasChildUnorphaned {
		t.Events.BlockUnorphaned.Trigger(currentBlock)
	}

	return currentBlock.Children()
}

// walkFutureCone traverses the future cone of the given Block and calls the given callback for each Block.
func (t *Tangle) walkFutureCone(block *Block, callback func(currentBlock *Block) (nextChildren []*Block)) {
	for childWalker := walker.New[*Block](false).PushAll(block.Children()...); childWalker.HasNext(); {
		childWalker.PushAll(callback(childWalker.Next())...)
	}
}

// checkReference checks if the reference between the child and its parent is valid.
func checkReference(child *Block, parent *Block) (err error) {
	if parent.invalid {
		return errors.Errorf("parent %s of child %s is marked as invalid", parent.ID(), child.ID())
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
