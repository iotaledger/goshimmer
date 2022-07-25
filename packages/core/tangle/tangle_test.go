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
		blockCount  = 2000
		tangleWidth = 250
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
	i := 0
	createNewBlock := func() *Block {
		// issue the payload
		strongParents := make([]string, 0)
		for _, selectedTip := range tips.RandomUniqueEntries(2) {
			if selectedTip == EmptyBlockID {
				strongParents = append(strongParents, "Genesis")
				continue
			}
			strongParents = append(strongParents, selectedTip.Alias())
		}
		blk := testFramework.CreateBlock(fmt.Sprintf("msg-%d", i), WithStrongParents(strongParents...))
		i++
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
	for i = 0; i < blockCount; i++ {
		blk := createNewBlock()
		blocks[blk.ID()] = blk
	}

	// counter for the different stages
	var (
		missingBlocks int32
		solidBlocks   int32
	)

	// increase the counter when a missing block was detected
	tangle.Events.BlockMissing.Attach(event.NewClosure(func(metadata *BlockMetadata) {
		atomic.AddInt32(&missingBlocks, 1)
		// store the block after it has been requested
		//go func() {
		time.Sleep(storeDelay)
		tangle.AttachBlock(blocks[metadata.id])
		//}()
	}))

	// decrease the counter when a missing block was received
	tangle.Events.MissingBlockStored.Hook(event.NewClosure(func(_ *BlockMetadata) {
		//n := atomic.AddInt32(&missingBlocks, -1)
		//t.Logf("missing blocks %d", n)
	}))

	tangle.Events.BlockSolid.Hook(event.NewClosure(func(_ *BlockMetadata) {
		n := atomic.AddInt32(&solidBlocks, 1)
		t.Logf("solid blocks %d/%d", n, blockCount)
	}))

	// issue tips to start solidification
	tips.ForEach(func(key BlockID, _ BlockID) { tangle.AttachBlock(blocks[key]) })

	// wait until all blocks are solidified
	event.Loop.WaitUntilAllTasksProcessed()
	assert.Eventually(t, func() bool { return atomic.LoadInt32(&solidBlocks) == blockCount }, 3*time.Second, 100*time.Millisecond)

	assert.EqualValues(t, blockCount, atomic.LoadInt32(&solidBlocks))
	assert.EqualValues(t, 0, atomic.LoadInt32(&missingBlocks))
}
