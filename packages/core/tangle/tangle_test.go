package tangle

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/randommap"
	"github.com/stretchr/testify/assert"
)

func TestTangleAttach(t *testing.T) {
	tangle := NewTangle(func(t *Tangle) {
		t.dbManagerPath = "/tmp/"
	})

	testFramework := NewBlockTestFramework(tangle)

	block1 := testFramework.CreateBlock("msg1", WithStrongParents("Genesis"))

	block2 := testFramework.CreateBlock("msg2", WithStrongParents("msg1"))

	tangle.Events.BlockMissing.Hook(event.NewClosure[*BlockMetadata](func(metadata *BlockMetadata) {
		t.Logf("block %s is missing", metadata.id)
	}))

	tangle.Events.BlockSolid.Hook(event.NewClosure[*BlockMetadata](func(metadata *BlockMetadata) {
		t.Logf("block %s is solid", metadata.id)
	}))

	tangle.Events.MissingBlockStored.Hook(event.NewClosure[*BlockMetadata](func(metadata *BlockMetadata) {
		t.Logf("missing block %s is stored", metadata.id)
	}))

	tangle.AttachBlock(block2)
	tangle.AttachBlock(block1)

	event.Loop.WaitUntilAllTasksProcessed()
}

func TestTangle_AttachBlock(t *testing.T) {
	blockTangle := NewTestTangle()
	defer blockTangle.Shutdown()

	blockTangle.Events.BlockSolid.Hook(event.NewClosure(func(metadata *BlockMetadata) {
		fmt.Println("SOLID:", metadata.id)
	}))

	blockTangle.Events.BlockMissing.Hook(event.NewClosure(func(metadata *BlockMetadata) {
		fmt.Println("MISSING:", metadata.id)
	}))

	blockTangle.Events.MissingBlockStored.Hook(event.NewClosure(func(metadata *BlockMetadata) {
		fmt.Println("REMOVED:", metadata.id)
	}))
	blockTangle.Events.BlockInvalid.Hook(event.NewClosure(func(metadata *BlockMetadata) {
		fmt.Println("INVALID:", metadata.id)
	}))

	newBlockOne := newTestDataBlock("some data")
	newBlockTwo := newTestDataBlock("some other data")

	blockTangle.AttachBlock(newBlockTwo)

	event.Loop.WaitUntilAllTasksProcessed()

	blockTangle.AttachBlock(newBlockOne)
}

func TestTangle_MissingBlocks(t *testing.T) {
	const (
		blockCount  = 1000
		tangleWidth = 50
		storeDelay  = 5 * time.Millisecond
	)

	// create the tangle
	tangle := NewTestTangle()
	defer tangle.Shutdown()
	testFramework := NewBlockTestFramework(tangle)

	// map to keep track of the tips
	tips := randommap.New[BlockID, BlockID]()
	tips.Set(EmptyBlockID, EmptyBlockID)

	// create a helper function that creates the blocks
	createNewBlock := func(idx int) *Block {
		// issue the payload
		strongParents := make([]string, 0)
		for _, selectedTip := range tips.RandomUniqueEntries(2) {
			if selectedTip == EmptyBlockID {
				strongParents = append(strongParents, "Genesis")
				continue
			}
			strongParents = append(strongParents, selectedTip.Alias())
		}
		blk := testFramework.CreateBlock(fmt.Sprintf("msg-%d", idx), WithStrongParents(strongParents...))
		// remove a tip if the width of the tangle is reached
		if tips.Size() >= tangleWidth {
			tips.Delete(blk.ParentsByType(StrongParentType).First())
		}

		// add current block as a tip
		tips.Set(blk.ID(), blk.ID())

		// return the constructed block
		return blk
	}

	// generate the blocks we want to solidify
	blocks := make(map[BlockID]*Block, blockCount)
	for i := 0; i < blockCount; i++ {
		blk := createNewBlock(i)
		blocks[blk.ID()] = blk
	}
	fmt.Println("blocks generated", len(blocks), "tip pool size", tips.Size())

	missingBlocks := int32(0)
	tangle.Events.MissingBlockStored.Hook(event.NewClosure(func(metadata *BlockMetadata) {
		t.Logf("missing blocks %d, %s", atomic.AddInt32(&missingBlocks, -1), metadata.id)
	}))

	storedBlocks := int32(0)
	tangle.Events.BlockStored.Hook(event.NewClosure(func(metadata *BlockMetadata) {
		t.Logf("stored blocks %d, %s", atomic.AddInt32(&storedBlocks, 1), metadata.id)
	}))

	solidBlocks := int32(0)
	tangle.Events.BlockSolid.Hook(event.NewClosure(func(metadata *BlockMetadata) {
		t.Logf("solid blocks %d/%d, %s", atomic.AddInt32(&solidBlocks, 1), blockCount, metadata.id)
	}))

	tangle.Events.BlockMissing.Attach(event.NewClosure(func(metadata *BlockMetadata) {
		atomic.AddInt32(&missingBlocks, 1)

		time.Sleep(storeDelay)

		tangle.AttachBlock(blocks[metadata.id])
	}))

	// issue tips to start solidification
	tips.ForEach(func(key BlockID, _ BlockID) {
		tangle.AttachBlock(blocks[key])
	})

	// wait until all blocks are solidified
	event.Loop.WaitUntilAllTasksProcessed()

	for blockID, _ := range blocks {
		metadata, _ := tangle.BlockMetadata(blockID)
		solidParents := 0
		if !metadata.solid {
			for parentID := range metadata.strongParents {
				parentMetadata, _ := tangle.BlockMetadata(parentID)
				if parentMetadata.solid {
					solidParents++
				}
			}
		}
		if solidParents == len(metadata.strongParents) {
			fmt.Println("block not solid but should be", metadata.id, metadata.solid)
		}
	}

	assert.EqualValues(t, blockCount, atomic.LoadInt32(&storedBlocks))
	assert.EqualValues(t, blockCount, atomic.LoadInt32(&solidBlocks))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingBlocks))
}
