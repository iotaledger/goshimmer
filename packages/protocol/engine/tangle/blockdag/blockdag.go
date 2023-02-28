package blockdag

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region BlockDAG /////////////////////////////////////////////////////////////////////////////////////////////////////

// BlockDAG is a causally ordered DAG that forms the central data structure of the IOTA protocol.
type BlockDAG struct {
	// Events contains the Events of the BlockDAG.
	Events *Events

	// EvictionState contains information about the current eviction state.
	EvictionState *eviction.State

	// memStorage contains the in-memory storage of the BlockDAG.
	memStorage *memstorage.EpochStorage[models.BlockID, *Block]

	// solidifier contains the solidifier instance used to determine the solidity of Blocks.
	solidifier *causalorder.CausalOrder[models.BlockID, *Block]

	// commitmentFunc is a function that returns the commitment corresponding to the given epoch index.
	commitmentFunc func(index epoch.Index) (*commitment.Commitment, error)

	// futureBlocks contains blocks with a commitment in the future, that should not be passed to the booker yet.
	futureBlocks *memstorage.EpochStorage[commitment.ID, *advancedset.AdvancedSet[*Block]]

	nextIndexToPromote epoch.Index

	// The Queue always read-locks the eviction mutex of the solidifier, and then evaluates if the block is
	// future thus read-locking the futureBlocks mutex. At the same time, when re-adding parked blocks,
	// promoteFutureBlocksMethod write-locks the futureBlocks mutex, and then read-locks the eviction mutex
	// of  the solidifer. As the locks are non-starving, and locks are interlocked in different orders a
	// deadlock can occur only when an eviction is triggered while the above scenario unfolds.
	solidifierMutex sync.RWMutex

	futureBlocksMutex sync.RWMutex

	// evictionMutex is a mutex that is used to synchronize the eviction of elements from the BlockDAG.
	evictionMutex sync.RWMutex

	Workers    *workerpool.Group
	workerPool *workerpool.WorkerPool
}

// New is the constructor for the BlockDAG and creates a new BlockDAG instance.
func New(workers *workerpool.Group, evictionState *eviction.State, latestCommitmentFunc func(epoch.Index) (*commitment.Commitment, error), opts ...options.Option[BlockDAG]) (newBlockDAG *BlockDAG) {
	return options.Apply(&BlockDAG{
		Events:         NewEvents(),
		EvictionState:  evictionState,
		memStorage:     memstorage.NewEpochStorage[models.BlockID, *Block](),
		commitmentFunc: latestCommitmentFunc,
		futureBlocks:   memstorage.NewEpochStorage[commitment.ID, *advancedset.AdvancedSet[*Block]](),
		Workers:        workers,
		workerPool:     workers.CreatePool("Solidifier", 2),
	}, opts, func(b *BlockDAG) {
		b.solidifier = causalorder.New(
			b.workerPool,
			b.Block,
			(*Block).IsSolid,
			b.markSolid,
			b.markInvalid,
			causalorder.WithReferenceValidator[models.BlockID](checkReference),
		)

		evictionState.Events.EpochEvicted.Hook(b.evictEpoch)
	})
}

// Attach is used to attach new Blocks to the BlockDAG. It is the main function of the BlockDAG that triggers Events.
func (b *BlockDAG) Attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	if block, wasAttached, err = b.attach(data); wasAttached {
		b.Events.BlockAttached.Trigger(block)

		b.solidifierMutex.RLock()
		defer b.solidifierMutex.RUnlock()

		b.solidifier.Queue(block)
	}

	return
}

// Block retrieves a Block with metadata from the in-memory storage of the BlockDAG.
func (b *BlockDAG) Block(id models.BlockID) (block *Block, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.block(id)
}

// SetInvalid marks a Block as invalid and propagates the invalidity to its future cone.
func (b *BlockDAG) SetInvalid(block *Block, reason error) (wasUpdated bool) {
	if wasUpdated = block.setInvalid(); wasUpdated {
		b.Events.BlockInvalid.Trigger(&BlockInvalidEvent{
			Block:  block,
			Reason: reason,
		})

		b.walkFutureCone(block.Children(), func(currentBlock *Block) []*Block {
			if !currentBlock.setInvalid() {
				return nil
			}

			b.Events.BlockInvalid.Trigger(&BlockInvalidEvent{
				Block:  currentBlock,
				Reason: reason,
			})

			return currentBlock.Children()
		})
	}

	return
}

// SetOrphaned marks a Block as orphaned and propagates it to its future cone.
func (b *BlockDAG) SetOrphaned(block *Block, orphaned bool) (updated bool) {
	// TODO: should we check whether the parents are not orphaned as well in case we unorphan?
	//  should not happen because we only unorphan in the blockgadget in the causal order which should happen in-order.

	if !block.setOrphaned(orphaned) {
		return
	}

	if orphaned {
		b.Events.BlockOrphaned.Trigger(block)
	} else {
		b.Events.BlockUnorphaned.Trigger(block)
	}

	return true
}

func (b *BlockDAG) PromoteFutureBlocksUntil(index epoch.Index) {
	b.solidifierMutex.RLock()
	defer b.solidifierMutex.RUnlock()

	b.futureBlocksMutex.Lock()
	defer b.futureBlocksMutex.Unlock()

	for i := b.nextIndexToPromote; i <= index; i++ {
		cm, err := b.commitmentFunc(i)
		if err != nil {
			panic(fmt.Sprintf("failed to load commitment for index %d: %s", i, err))
		}
		if storage := b.futureBlocks.Get(i, false); storage != nil {
			if futureBlocks, exists := storage.Get(cm.ID()); exists {
				_ = futureBlocks.ForEach(func(futureBlock *Block) (err error) {
					b.solidifier.Queue(futureBlock)
					return nil
				})
			}
		}
		b.futureBlocks.Evict(i)
	}

	b.nextIndexToPromote = index + 1
}

// evictEpoch is used to evict Blocks from committed epochs from the BlockDAG.
func (b *BlockDAG) evictEpoch(index epoch.Index) {
	b.solidifierMutex.Lock()
	defer b.solidifierMutex.Unlock()

	b.solidifier.EvictUntil(index)

	b.evictionMutex.Lock()
	defer b.evictionMutex.Unlock()

	b.memStorage.Evict(index)
}

func (b *BlockDAG) markSolid(block *Block) (err error) {
	// Future blocks already have passed these checks, as they are revisited again at a later point in time.
	// It is important to note that this check only passes once for a specific future block, as it is not yet marked as
	// such. The next time the block is added to the causal orderer and marked as solid (that is, when it got promoted),
	// it will be marked solid straight away, without checking if it is still a future block: that's why this method
	// must be only called at most twice for a any block.
	if !block.IsFuture() {
		if err := b.checkParents(block); err != nil {
			return err
		}

		if b.isFutureBlock(block) {
			return
		}
	}

	// It is important to only set the block as solid when it was not "parked" as a future block.
	// Future blocks are queued for solidification again when the epoch is committed.
	block.setSolid()

	b.Events.BlockSolid.Trigger(block)

	return nil
}

func (b *BlockDAG) isFutureBlock(block *Block) (isFutureBlock bool) {
	b.futureBlocksMutex.RLock()
	defer b.futureBlocksMutex.RUnlock()

	// If we are not able to load the commitment for the block, it means we haven't committed this epoch yet.
	if _, err := b.commitmentFunc(block.Commitment().Index()); err != nil {
		// We set the block as future block so that we can skip some checks when revisiting it later in markSolid via the solidifier.
		block.setFuture()

		lo.Return1(b.futureBlocks.Get(block.Commitment().Index(), true).GetOrCreate(block.Commitment().ID(), func() *advancedset.AdvancedSet[*Block] {
			return advancedset.NewAdvancedSet[*Block]()
		})).Add(block)
		return true
	}

	return false
}

func (b *BlockDAG) checkParents(block *Block) (err error) {
	for _, parentID := range block.Parents() {
		parent, parentExists := b.Block(parentID)
		if !parentExists {
			panic(fmt.Sprintf("parent %s of block %s should exist as block was marked ordered by the solidifier", parentID, block.ID()))
		}

		// check timestamp monotonicity
		if parent.IssuingTime().After(block.IssuingTime()) {
			return errors.Errorf("timestamp monotonicity check failed for parent %s with timestamp %s. block timestamp %s", parent.ID(), parent.IssuingTime(), block.IssuingTime())
		}

		// check commitment monotonicity
		if parent.Commitment().Index() > block.Commitment().Index() {
			return errors.Errorf("commitment monotonicity check failed for parent %s with commitment index %d. block commitment index %d", parentID, parent.Commitment().Index(), block.Commitment().Index())
		}
	}

	return nil
}

func (b *BlockDAG) markInvalid(block *Block, reason error) {
	b.SetInvalid(block, errors.Wrap(reason, "block marked as invalid in BlockDAG"))
}

// attach tries to attach the given Block to the BlockDAG.
func (b *BlockDAG) attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	if block, wasAttached, err = b.canAttach(data); !wasAttached {
		return
	}

	if block, wasAttached = b.memStorage.Get(data.ID().EpochIndex, true).GetOrCreate(data.ID(), func() *Block { return NewBlock(data) }); !wasAttached {
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
	if b.EvictionState.InEvictedEpoch(data.ID()) && !b.EvictionState.IsRootBlock(data.ID()) {
		return nil, false, errors.Errorf("block data with %s is too old (issued at: %s)", data.ID(), data.IssuingTime())
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
		if b.EvictionState.InEvictedEpoch(parentID) && !b.EvictionState.IsRootBlock(parentID) {
			if storedBlock != nil {
				b.SetInvalid(storedBlock, errors.Errorf("block with %s references too old parent %s", data.ID(), parentID))
			}

			return storedBlock, false, errors.Errorf("parent %s of block %s is too old", parentID, data.ID())
		}
	}

	return storedBlock, true, nil
}

// registerChild registers the given Block as a child of the parent. It triggers a BlockMissing event if the referenced
// Block does not exist, yet.
func (b *BlockDAG) registerChild(child *Block, parent models.Parent) {
	if b.EvictionState.IsRootBlock(parent.ID) {
		return
	}

	parentBlock, _ := b.memStorage.Get(parent.ID.Index(), true).GetOrCreate(parent.ID, func() (newBlock *Block) {
		newBlock = NewBlock(models.NewEmptyBlock(parent.ID), WithMissing(true))

		b.Events.BlockMissing.Trigger(newBlock)

		return
	})

	parentBlock.appendChild(child, parent.Type)
}

// block retrieves the Block with given id from the mem-storage.
func (b *BlockDAG) block(id models.BlockID) (block *Block, exists bool) {
	if b.EvictionState.IsRootBlock(id) {
		return NewRootBlock(id), true
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

// checkReference checks if the reference between the child and its parent is valid.
func checkReference(child *Block, parent *Block) (err error) {
	if parent.invalid {
		return errors.Errorf("parent %s of child %s is marked as invalid", parent.ID(), child.ID())
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
