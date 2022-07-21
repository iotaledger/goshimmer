package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

var GenesisMetadata = &BlockMetadata{
	id:                   EmptyBlockID,
	strongParents:        make(BlockIDs),
	weakParents:          make(BlockIDs),
	likedInsteadParents:  make(BlockIDs),
	strongChildren:       make([]*BlockMetadata, 0),
	weakChildren:         make([]*BlockMetadata, 0),
	likedInsteadChildren: make([]*BlockMetadata, 0),
	solid:                true,
	transactionMutex:     syncutils.NewStarvingMutex(),
}

type Tangle struct {
	Events          Events
	metadataStorage *memstorage.EpochStorage[BlockID, *BlockMetadata]
	dbManager       *database.Manager
}

func (t *Tangle) Attach(block *Block) {
	// abort if too old
	if false {
		return
	}
	blockMetadata, created := t.createBlockMetadata(block)
	if !created {
		return
	}

	err := t.dbManager.Get(block.ID().EpochIndex, []byte{tangleold.PrefixBlock}).Set(block.IDBytes(), lo.PanicOnErr(block.Bytes()))
	if err != nil {
		panic(err)
	}

	//transaction
	blockMetadata.Transaction(func() {
		missingParents := uint8(0)
		for parentID := range blockMetadata.ParentIDs() {
			parentMetadata, _ := t.GetBlockMetadata(parentID)
			parentMetadata.Transaction(func() {
				if parentMetadata.missing {
					missingParents++
				}
			})
		}
		blockMetadata.missingParents = missingParents
		if missingParents == 0 {
			blockMetadata.solid = true
			t.Events.BlockSolid.Trigger(&BlockSolidEvent{BlockMetadata: blockMetadata})
		}
	})
	//transaction
	fmt.Printf("blockmetadata %+v\n", blockMetadata)

}

func (t *Tangle) createBlockMetadata(block *Block) (blockMetadata *BlockMetadata, created bool) {

	epochStorage, _ := t.metadataStorage.Get(block.ID().EpochIndex, true)

	// lock
	blockMetadata, retrieved := epochStorage.RetrieveOrCreate(block.ID(), func() *BlockMetadata {
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
	})

	isNew := true
	blockMetadata.Transaction(func() {
		if retrieved {
			if !blockMetadata.missing {
				isNew = false
				return
			}
			blockMetadata.missing = false
			blockMetadata.strongParents = block.ParentsByType(StrongParentType)
			blockMetadata.weakParents = block.ParentsByType(WeakParentType)
			blockMetadata.likedInsteadParents = block.ParentsByType(ShallowLikeParentType)
			t.Events.MissingBlockStored.Trigger(&MissingBlockStoredEvent{BlockID: blockMetadata.id})
		}
	})
	if !isNew {
		return nil, false
	}
	t.registerAsChild(block, blockMetadata)

	return blockMetadata, true
}

func (t *Tangle) registerAsChild(block *Block, metadata *BlockMetadata) {
	for parentID := range block.ParentsByType(StrongParentType) {
		if parentID == EmptyBlockID {
			continue
		}
		parentEpochStorage, _ := t.metadataStorage.Get(parentID.EpochIndex, true)

		parentMetadata, _ := parentEpochStorage.RetrieveOrCreate(parentID, func() *BlockMetadata {
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
		if parentID == EmptyBlockID {
			continue
		}
		parentEpochStorage, _ := t.metadataStorage.Get(parentID.EpochIndex, true)

		parentMetadata, _ := parentEpochStorage.RetrieveOrCreate(parentID, func() *BlockMetadata {
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
		if parentID == EmptyBlockID {
			continue
		}
		parentEpochStorage, _ := t.metadataStorage.Get(parentID.EpochIndex, true)

		parentMetadata, _ := parentEpochStorage.RetrieveOrCreate(parentID, func() *BlockMetadata {
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

func (t *Tangle) GetBlockMetadata(blockID BlockID) (metadata *BlockMetadata, exists bool) {
	if blockID == EmptyBlockID {
		return GenesisMetadata, true
	}
	epochStorage, _ := t.metadataStorage.Get(blockID.EpochIndex, true)
	return epochStorage.Get(blockID)
}

// Events is the event interface of the Tangle.
type Events struct {
	// BlockSolid is triggered when a block becomes solid, i.e. its past cone is known and solid.
	BlockSolid *event.Event[*BlockSolidEvent]

	// BlockMissing is triggered when a block references an unknown parent Block.
	BlockMissing *event.Event[*BlockMissingEvent]

	// MissingBlockStored fired when a block which was previously marked as missing was received.
	MissingBlockStored *event.Event[*MissingBlockStoredEvent]
}

func NewEvents() *Events {
	events := &Events{
		BlockSolid:         event.New[*BlockSolidEvent](),
		BlockMissing:       event.New[*BlockMissingEvent](),
		MissingBlockStored: event.New[*MissingBlockStoredEvent](),
	}
	return events
}

type BlockSolidEvent struct {
	BlockMetadata *BlockMetadata
}

type BlockMissingEvent struct {
	BlockID BlockID
}
type MissingBlockStoredEvent struct {
	BlockID BlockID
}
