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

	dbManager       *database.Manager
	dbManagerPath   string
	memStorage      *memstorage.EpochStorage[models.BlockID, *Block]
	maxDroppedEpoch epoch.Index
	solidifier      *causalorder.CausalOrder[models.BlockID, *Block]
	genesisBlock    func(models.BlockID) *Block

	sync.RWMutex
}

// New is the constructor for the Tangle.
func New(opts ...options.Option[Tangle]) (newTangle *Tangle) {
	return options.Apply(&Tangle{
		Events:        newEvents(),
		memStorage:    memstorage.NewEpochStorage[models.BlockID, *Block](),
		dbManagerPath: "/tmp/",
		genesisBlock:  defaultGenesisBlockProvider,
	}, opts).init()
}

// Attach is used to attach new Blocks to the Tangle. This function also triggers the necessary events.
func (t *Tangle) Attach(data *models.Block) (block *Block, isNew bool) {
	t.RLock()
	defer t.RUnlock()

	if t.isTooOld(data) {
		// TODO: propagate invalidity to any existing future-cone of the block
		return
	}

	if block, isNew = t.publishBlockData(data); !isNew {
		return
	}

	t.Events.BlockStored.Trigger(block)

	t.solidifier.Queue(block)

	return
}

// Block retrieves a Block with metadata from the cache.
func (t *Tangle) Block(id models.BlockID) (block *Block, exists bool) {
	t.RLock()
	defer t.RUnlock()

	return t.block(id)
}

// SetInvalid is used to mark a Block as invalid and propagate invalidity to its future cone. Locks the metadata mutex.
func (t *Tangle) SetInvalid(block *Block) (wasUpdated bool) {
	if wasUpdated = block.setInvalid(); !wasUpdated {
		return
	}

	t.Events.BlockInvalid.Trigger(block)

	t.propagateInvalidityToChildren(block.Children())

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

// Shutdown marks the tangle as stopped, so it will not accept any new blocks (waits for all backgroundTasks to finish).
func (t *Tangle) Shutdown() {
	t.dbManager.Shutdown()
}

func (t *Tangle) init() (self *Tangle) {
	t.dbManager = database.NewManager(t.dbManagerPath)

	t.solidifier = causalorder.New(
		t.Block,
		func(block *Block) (isSolid bool) {
			return block.solid
		},
		(*Block).setSolid,
		t.genesisBlock,
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

func (t *Tangle) publishBlockData(data *models.Block) (block *Block, published bool) {
	block, published = t.epochStorage(data.ID()).RetrieveOrCreate(data.ID(), func() *Block {
		return NewBlock(data)
	})

	if !published {
		if !block.publishBlockData(data) {
			return block, false
		}

		t.Events.MissingBlockStored.Trigger(block)
	}

	t.storeBlock(data)

	block.ForEachParent(func(parent models.Parent) {
		t.registerChild(block, parent)
	})

	return block, true
}

func (t *Tangle) storeBlock(block *models.Block) {
	if err := t.dbManager.Get(block.ID().EpochIndex, []byte{tangleold.PrefixBlock}).Set(block.IDBytes(), lo.PanicOnErr(block.Bytes())); err != nil {
		t.Events.Error.Trigger(errors.Errorf("failed to store block with %s on disk: %w", block.ID(), err))
	}
}

func (t *Tangle) registerChild(block *Block, parent models.Parent) {
	if t.isGenesisBlock(parent.ID) {
		return
	}

	parentBlock, _ := t.epochStorage(parent.ID).RetrieveOrCreate(parent.ID, func() (newBlock *Block) {
		newBlock = NewBlock(models.NewEmptyBlock(parent.ID), WithMissing(true))

		t.Events.BlockMissing.Trigger(newBlock)

		return
	})

	parentBlock.appendChild(block, parent.Type)
}

func (t *Tangle) block(blockID models.BlockID) (block *Block, exists bool) {
	if block = t.genesisBlock(blockID); block != nil {
		return block, true
	}

	if t.isBlockIDTooOld(blockID) {
		return nil, false
	}

	return t.epochStorage(blockID).Get(blockID)
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
	return !t.isGenesisBlock(blockID) && blockID.EpochIndex <= t.maxDroppedEpoch
}

func (t *Tangle) isGenesisBlock(id models.BlockID) (isGenesisBlock bool) {
	return t.genesisBlock(id) != nil
}

func (t *Tangle) epochStorage(blockID models.BlockID) (epochStorage *memstorage.Storage[models.BlockID, *Block]) {
	return t.memStorage.Get(blockID.EpochIndex, true)
}
