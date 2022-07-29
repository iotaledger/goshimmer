package tangle

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/randommap"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

func TestTangleAttach(t *testing.T) {
	tangle := New(database.NewManager(t.TempDir()))

	testFramework := NewBlockTestFramework(tangle)

	block1 := testFramework.CreateBlock("msg1", WithStrongParents("Genesis"))

	block2 := testFramework.CreateBlock("msg2", WithStrongParents("msg1"))

	tangle.Events.BlockMissing.Hook(event.NewClosure[*Block](func(metadata *Block) {
		t.Logf("block %s is missing", metadata.ID())
	}))

	tangle.Events.BlockSolid.Hook(event.NewClosure[*Block](func(metadata *Block) {
		t.Logf("block %s is solid", metadata.ID())
	}))

	tangle.Events.MissingBlockAttached.Hook(event.NewClosure[*Block](func(metadata *Block) {
		t.Logf("missing block %s is stored", metadata.ID())
	}))

	tangle.Attach(block2)
	tangle.Attach(block1)

	event.Loop.WaitUntilAllTasksProcessed()
}

func TestTangle_AttachBlock(t *testing.T) {
	blockTangle := New(database.NewManager(t.TempDir()))
	defer blockTangle.Shutdown()

	blockTangle.Events.BlockSolid.Hook(event.NewClosure(func(metadata *Block) {
		fmt.Println("SOLID:", metadata.ID())
	}))

	blockTangle.Events.BlockMissing.Hook(event.NewClosure(func(metadata *Block) {
		fmt.Println("MISSING:", metadata.ID())
	}))

	blockTangle.Events.MissingBlockAttached.Hook(event.NewClosure(func(metadata *Block) {
		fmt.Println("REMOVED:", metadata.ID())
	}))
	blockTangle.Events.BlockInvalid.Hook(event.NewClosure(func(metadata *Block) {
		fmt.Println("INVALID:", metadata.ID())
	}))

	newBlockOne := newTestDataBlock("some data")
	newBlockTwo := newTestDataBlock("some other data")

	blockTangle.Attach(newBlockTwo)

	event.Loop.WaitUntilAllTasksProcessed()

	blockTangle.Attach(newBlockOne)
}

func TestTangle_MissingBlocks(t *testing.T) {
	const (
		blockCount  = 10000
		tangleWidth = 500
		storeDelay  = 0 * time.Millisecond
	)

	// create the tangle
	tangle := New(database.NewManager(t.TempDir()))
	defer tangle.Shutdown()
	testFramework := NewBlockTestFramework(tangle)

	// map to keep track of the tips
	tips := randommap.New[models.BlockID, models.BlockID]()
	tips.Set(models.EmptyBlockID, models.EmptyBlockID)

	// create a helper function that creates the blocks
	createNewBlock := func(idx int) *models.Block {
		// issue the payload
		strongParents := make([]string, 0)
		for _, selectedTip := range tips.RandomUniqueEntries(2) {
			if selectedTip == models.EmptyBlockID {
				strongParents = append(strongParents, "Genesis")
				continue
			}
			strongParents = append(strongParents, selectedTip.Alias())
		}
		blk := testFramework.CreateBlock(fmt.Sprintf("msg-%d", idx), WithStrongParents(strongParents...))
		// remove a tip if the width of the tangle is reached
		if tips.Size() >= tangleWidth {
			tips.Delete(blk.ParentsByType(models.StrongParentType).First())
		}

		// add current block as a tip
		tips.Set(blk.ID(), blk.ID())

		// return the constructed block
		return blk
	}

	// generate the blocks we want to solidify
	blocks := make(map[models.BlockID]*models.Block, blockCount)
	for i := 0; i < blockCount; i++ {
		blk := createNewBlock(i)
		blocks[blk.ID()] = blk
	}
	// fmt.Println("blocks generated", len(blocks), "tip pool size", tips.Size())

	invalidBlocks := int32(0)
	tangle.Events.BlockInvalid.Hook(event.NewClosure(func(metadata *Block) {
		_ = atomic.AddInt32(&invalidBlocks, 1)
		// t.Logf("invalid blocks %d, %s", , metadata.ID())
	}))

	missingBlocks := int32(0)
	tangle.Events.MissingBlockAttached.Hook(event.NewClosure(func(metadata *Block) {
		_ = atomic.AddInt32(&missingBlocks, -1)
		// t.Logf("missing blocks %d, %s", n, metadata.ID())
	}))

	storedBlocks := int32(0)
	tangle.Events.BlockAttached.Hook(event.NewClosure(func(metadata *Block) {
		_ = atomic.AddInt32(&storedBlocks, 1)
		// t.Logf("stored blocks %d, %s", , metadata.ID())
	}))

	solidBlocks := int32(0)
	tangle.Events.BlockSolid.Hook(event.NewClosure(func(metadata *Block) {
		_ = atomic.AddInt32(&solidBlocks, 1)
		// t.Logf("solid blocks %d/%d, %s", , blockCount, metadata.ID())
	}))

	tangle.Events.BlockMissing.Attach(event.NewClosure(func(metadata *Block) {
		atomic.AddInt32(&missingBlocks, 1)

		time.Sleep(storeDelay)

		tangle.Attach(blocks[metadata.ID()])
	}))

	// issue tips to start solidification
	tips.ForEach(func(key models.BlockID, _ models.BlockID) {
		tangle.Attach(blocks[key])
	})

	// wait until all blocks are solidified
	event.Loop.WaitUntilAllTasksProcessed()

	for blockID := range blocks {
		metadata, _ := tangle.Block(blockID)
		solidParents := 0
		if !metadata.solid {
			for parentID := range metadata.ParentsByType(models.StrongParentType) {
				parentMetadata, _ := tangle.Block(parentID)
				if parentMetadata.solid {
					solidParents++
				}
			}
		}
		if solidParents == len(metadata.ParentsByType(models.StrongParentType)) {
			fmt.Println("block not solid but should be", metadata.ID(), metadata.solid)
		}
	}

	assert.EqualValues(t, blockCount, atomic.LoadInt32(&storedBlocks))
	assert.EqualValues(t, blockCount, atomic.LoadInt32(&solidBlocks))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingBlocks))
}
