package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

// region Tangle ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Tangle is a causally ordered DAG that forms the central data structure of the IOTA protocol.
type Tangle struct {
	// Events contains the Events of Tangle.
	Events *Events

	// dbManager contains the database manager instance used to store a Block on disk.
	dbManager *database.Manager

	// memStorage contains the in-memory storage of the Tangle.
	memStorage *memstorage.EpochStorage[models.BlockID, *Block]

	// maxDroppedEpoch contains the highest epoch.Index that has been dropped from the Tangle.
	maxDroppedEpoch epoch.Index

	// solidifier contains the solidifier instance used to determine the solidity of Blocks.
	solidifier *causalorder.CausalOrder[models.BlockID, *Block]

	// rootBlockProvider contains a function that is used to retrieve the root Blocks of the Tangle.
	rootBlockProvider func(models.BlockID) *Block

	// isShutdown contains a flag that indicates whether the Tangle was shut down.
	isShutdown bool

	// RWMutex is used to protect access to the internal settings of the Tangle (i.e. maxDroppedEpoch).
	sync.RWMutex
}

// New is the constructor for the Tangle and creates a new Tangle instance.
func New(dbManager *database.Manager, rootBlockProvider func(models.BlockID) *Block, opts ...options.Option[Tangle]) (newTangle *Tangle) {
	return options.Apply(&Tangle{
		Events:            newEvents(),
		dbManager:         dbManager,
		memStorage:        memstorage.NewEpochStorage[models.BlockID, *Block](),
		rootBlockProvider: rootBlockProvider,
		maxDroppedEpoch:   -1,
	}, opts).initSolidifier(
		causalorder.WithReferenceValidator[models.BlockID](isReferenceValid),
	)
}

// Attach is used to attach new Blocks to the Tangle. It is the main function of the Tangle that triggers Events.
func (t *Tangle) Attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	t.RLock()
	defer t.RUnlock()

	if t.isShutdown {
		return nil, false, errors.Errorf("tangle is shut down")
	}

	if block, wasAttached, err = t.attach(data); wasAttached {
		t.Events.BlockAttached.Trigger(block)

		t.solidifier.Queue(block)
	}

	return
}

// Block retrieves a Block with metadata from the in-memory storage of the Tangle.
func (t *Tangle) Block(id models.BlockID) (block *Block, exists bool) {
	t.RLock()
	defer t.RUnlock()

	if t.isShutdown {
		return nil, false
	}

	return t.block(id)
}

// SetInvalid marks a Block as invalid and propagates the invalidity to its future cone.
func (t *Tangle) SetInvalid(block *Block) (wasUpdated bool) {
	if t.IsShutdown() {
		return false
	}

	if wasUpdated = block.setInvalid(); wasUpdated {
		t.Events.BlockInvalid.Trigger(block)

		t.propagateInvalidity(block.Children())
	}

	return
}

// Prune is used to prune the Tangle of all Blocks that are too old.
func (t *Tangle) Prune(epochIndex epoch.Index) {
	t.solidifier.Prune(epochIndex)

	t.prune(epochIndex)
}

func (t *Tangle) prune(epochIndex epoch.Index) {
	t.Lock()
	defer t.Unlock()

	if t.isShutdown || epochIndex <= t.maxDroppedEpoch {
		return
	}

	for t.maxDroppedEpoch < epochIndex {
		t.maxDroppedEpoch++
		t.memStorage.Drop(t.maxDroppedEpoch)
	}
}

// Shutdown marks the tangle as stopped, so it will not accept any new blocks (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.Lock()
	defer t.Unlock()

	if !t.isShutdown {
		t.isShutdown = true
	}
}

// IsShutdown returns true if the Tangle was shut down before.
func (t *Tangle) IsShutdown() (isShutdown bool) {
	t.RLock()
	defer t.RUnlock()

	return t.isShutdown
}

// initSolidifier is used to lazily initialize the solidifier after the options have been populated.
func (t *Tangle) initSolidifier(opts ...options.Option[causalorder.CausalOrder[models.BlockID, *Block]]) (self *Tangle) {
	t.solidifier = causalorder.New(t.Block, (*Block).IsSolid, t.markSolid, opts...)
	t.solidifier.Events.Drop.Attach(event.NewClosure(func(block *Block) { t.SetInvalid(block) }))

	return t
}

func (t *Tangle) markSolid(block *Block) (err error) {
	block.setSolid()

	t.Events.BlockSolid.Trigger(block)

	return nil
}

// attach tries to attach the given Block to the Tangle.
func (t *Tangle) attach(data *models.Block) (block *Block, wasAttached bool, err error) {
	if block, wasAttached, err = t.canAttach(data); !wasAttached {
		return
	}

	if block, wasAttached = t.memStorage.Get(data.ID().EpochIndex, true).RetrieveOrCreate(data.ID(), func() *Block { return NewBlock(data) }); !wasAttached {
		if wasAttached = block.update(data); !wasAttached {
			return
		}

		t.Events.MissingBlockAttached.Trigger(block)
	}

	t.storeData(data)

	block.ForEachParent(func(parent models.Parent) {
		t.registerChild(block, parent)
	})

	return
}

// canAttach determines if the Block can be attached (does not exist and addresses a recent epoch).
func (t *Tangle) canAttach(data *models.Block) (block *Block, canAttach bool, err error) {
	if t.isTooOld(data.ID()) {
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
		if t.isTooOld(parentID) {
			if storedBlock != nil {
				t.SetInvalid(storedBlock)
			}

			return storedBlock, false, errors.Errorf("parent %s of block %s is too old", parentID, data.ID())
		}
	}

	return storedBlock, true, nil
}

// storeData stores the given Block on disk.
func (t *Tangle) storeData(block *models.Block) {
	if err := t.dbManager.Get(block.ID().EpochIndex, []byte{tangleold.PrefixBlock}).Set(block.IDBytes(), lo.PanicOnErr(block.Bytes())); err != nil {
		panic(errors.Errorf("failed to store block with %s on disk: %w", block.ID(), err))
	}
}

// registerChild registers the given Block as a child of the parent. It triggers a BlockMissing event if the referenced
// Block does not exist, yet.
func (t *Tangle) registerChild(child *Block, parent models.Parent) {
	if t.rootBlockProvider(parent.ID) != nil {
		return
	}

	parentBlock, _ := t.memStorage.Get(parent.ID.EpochIndex, true).RetrieveOrCreate(parent.ID, func() (newBlock *Block) {
		newBlock = NewBlock(models.NewEmptyBlock(parent.ID), WithMissing(true))

		t.Events.BlockMissing.Trigger(newBlock)

		return
	})

	parentBlock.appendChild(child, parent.Type)
}

// block retrieves the Block with given id from the mem-storage.
func (t *Tangle) block(id models.BlockID) (block *Block, exists bool) {
	if block = t.rootBlockProvider(id); block != nil {
		return block, true
	}

	if t.isTooOld(id) {
		return nil, false
	}

	return t.memStorage.Get(id.EpochIndex, true).Get(id)
}

// propagateInvalidity marks the children (and their future cone) as invalid.
func (t *Tangle) propagateInvalidity(children []*Block) {
	for childWalker := walker.New[*Block](true).PushAll(children...); childWalker.HasNext(); {
		if child := childWalker.Next(); child.setInvalid() {
			t.Events.BlockInvalid.Trigger(child)

			childWalker.PushAll(child.Children()...)
		}
	}
}

// isTooOld checks if the Block associated with the given id is too old (in a pruned epoch).
func (t *Tangle) isTooOld(id models.BlockID) (isTooOld bool) {
	return id.EpochIndex <= t.maxDroppedEpoch && t.rootBlockProvider(id) == nil
}

// isReferenceValid checks if the reference between the child and its parent is valid.
func isReferenceValid(child *Block, parent *Block) (isValid bool) {
	return !parent.invalid
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
