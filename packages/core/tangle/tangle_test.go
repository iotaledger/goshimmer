package tangle

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/randommap"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/node/database"
)

func TestTangle_AttachBlock(t *testing.T) {
	blockTangle := New(database.NewManager(t.TempDir()))
	defer blockTangle.Shutdown()
	testFramework := models.NewTestFramework()

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

	newBlockOne := testFramework.CreateBlock("block1", models.WithStrongParents("Genesis"))
	newBlockTwo := testFramework.CreateBlock("block2", models.WithStrongParents("Genesis"))

	blockTangle.Attach(newBlockTwo)

	event.Loop.WaitUntilAllTasksProcessed()

	blockTangle.Attach(newBlockOne)
}

func TestTangle_Shutdown(t *testing.T) {
	db := database.NewManager(t.TempDir())
	blockTangle := New(db)
	blockTangle.Shutdown()

	testFramework := models.NewTestFramework()

	newBlock := testFramework.CreateBlock("block", models.WithStrongParents("Genesis"))

	_, _, err := blockTangle.Attach(newBlock)
	assert.Error(t, err, "should not be able to attach a block after shutdown")

	_, exists := blockTangle.Block(newBlock.ID())
	assert.False(t, exists, "block should not be in the tangle")

	wasUpdated := blockTangle.SetInvalid(&Block{})
	assert.False(t, wasUpdated, "block should not be updated")
}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When pruning the tangle, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func TestTangle_AttachInvalid(t *testing.T) {
	t.Skip()
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	db := database.NewManager(t.TempDir())
	blockTangle := New(db)
	defer blockTangle.Shutdown()

	solidBlocks := int32(0)
	blockTangle.Events.BlockSolid.Hook(event.NewClosure(func(metadata *Block) {
		atomic.AddInt32(&solidBlocks, 1)
	}))

	invalidBlocks := int32(0)
	blockTangle.Events.BlockInvalid.Hook(event.NewClosure(func(metadata *Block) {
		atomic.AddInt32(&invalidBlocks, 1)
	}))

	testFramework := models.NewTestFramework()

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) *models.Block {
		if idx == 0 {
			return testFramework.CreateBlock(fmt.Sprintf("blk%s-%d", prefix, idx), models.WithStrongParents("Genesis"), models.WithBlockIssuingTime(time.Unix(epoch.GenesisTime, 0)))
		}
		return testFramework.CreateBlock(fmt.Sprintf("blk%s-%d", prefix, idx), models.WithStrongParents(fmt.Sprintf("blk%s-%d", prefix, idx-1)), models.WithBlockIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)))
	}

	assert.EqualValues(t, 0, blockTangle.maxDroppedEpoch, "maxDroppedEpoch should be 0")
	blockTangle.Prune(epochCount / 2)
	assert.EqualValues(t, epochCount/2, blockTangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/2")
	event.Loop.WaitUntilAllTasksProcessed()

	blocks := make([]*models.Block, epochCount)
	// Attach solid blocks
	for i := 0; i < epochCount; i++ {
		blocks[i] = createNewBlock(i, "")
	}

	// Attach a blocks that are not solid (skip the first one in the chain
	for i := len(blocks) - 1; i >= 0; i-- {
		fmt.Println("Attaching block", blocks[i].ID())
		_, wasAttached, err := blockTangle.Attach(blocks[i])
		if blocks[i].ID().Index() > blockTangle.maxDroppedEpoch {
			assert.True(t, wasAttached, "block should be attached")
			assert.NoError(t, err, "should not be able to attach a block after shutdown")
			continue
		}
		assert.False(t, wasAttached, "block should not be attached")
		assert.Error(t, err, "should not be able to attach a block to a pruned epoch")
	}
	event.Loop.WaitUntilAllTasksProcessed()
	assert.EqualValues(t, 0, atomic.LoadInt32(&solidBlocks), "all blocks should be solid")
	assert.EqualValues(t, epochCount/2, atomic.LoadInt32(&invalidBlocks), "all blocks should be solid")

}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When pruning the tangle, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func TestTangle_Prune(t *testing.T) {
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	db := database.NewManager(t.TempDir())
	blockTangle := New(db)
	defer blockTangle.Shutdown()

	solidBlocks := int32(0)
	blockTangle.Events.BlockSolid.Hook(event.NewClosure(func(metadata *Block) {
		fmt.Println("SOLID:", metadata.ID())
		atomic.AddInt32(&solidBlocks, 1)
	}))

	invalidBlocks := int32(0)
	blockTangle.Events.BlockInvalid.Hook(event.NewClosure(func(metadata *Block) {
		fmt.Println("INVALID:", metadata.ID())
		atomic.AddInt32(&invalidBlocks, 1)
	}))

	testFramework := models.NewTestFramework()

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) *models.Block {
		if idx == 0 {
			return testFramework.CreateBlock(fmt.Sprintf("blk%s-%d", prefix, idx), models.WithStrongParents("Genesis"), models.WithBlockIssuingTime(time.Unix(epoch.GenesisTime, 0)))
		}
		return testFramework.CreateBlock(fmt.Sprintf("blk%s-%d", prefix, idx), models.WithStrongParents(fmt.Sprintf("blk%s-%d", prefix, idx-1)), models.WithBlockIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)))
	}

	assert.EqualValues(t, -1, blockTangle.maxDroppedEpoch, "maxDroppedEpoch should be 0")

	// Attach solid blocks
	for i := 0; i < epochCount; i++ {
		_, wasAttached, err := blockTangle.Attach(createNewBlock(i, ""))
		assert.True(t, wasAttached, "block should be attached")
		assert.NoError(t, err, "should not be able to attach a block after shutdown")
	}

	// Attach a blocks that are not solid (skip the first one in the chain
	for i := 0; i < epochCount; i++ {
		blk := createNewBlock(i, "-orphan")
		if i == 0 {
			continue
		}
		_, wasAttached, err := blockTangle.Attach(blk)
		assert.True(t, wasAttached, "block should be attached")
		assert.NoError(t, err, "should not be able to attach a block after shutdown")
	}

	event.Loop.WaitUntilAllTasksProcessed()
	assert.EqualValues(t, int32(epochCount), atomic.LoadInt32(&solidBlocks), "all blocks should be solid")

	_, exists := blockTangle.Block(testFramework.Block("blk-0").ID())
	assert.True(t, exists, "block should be in the tangle")

	blockTangle.Prune(epochCount / 4)
	event.Loop.WaitUntilAllTasksProcessed()

	assert.EqualValues(t, epochCount/4, blockTangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/4")

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	assert.EqualValues(t, int32(100), atomic.LoadInt32(&invalidBlocks), "orphaned blocks should be invalid")

	blockTangle.Prune(epochCount / 10)
	assert.EqualValues(t, epochCount/4, blockTangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/4")

	blockTangle.Prune(epochCount / 2)
	assert.EqualValues(t, epochCount/2, blockTangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/2")

	_, exists = blockTangle.Block(testFramework.Block("blk-0").ID())
	assert.False(t, exists, "block should not be in the tangle")
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
	testFramework := models.NewTestFramework()

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
		blk := testFramework.CreateBlock(fmt.Sprintf("msg-%d", idx), models.WithStrongParents(strongParents...))
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
