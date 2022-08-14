package tangle

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// This test checks if the internal metadata is correct i.e. that children are assigned correctly and that all the flags are correct.
func TestTangle_AttachBlock(t *testing.T) {
	tf := NewTestFramework(t)
	defer tf.Shutdown()
	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))
	tf.CreateBlock("block3", models.WithStrongParents(tf.BlockIDs("block1", "block2")))
	tf.CreateBlock("block4", models.WithStrongParents(tf.BlockIDs("block3", "block2")))
	tf.CreateBlock("block5", models.WithStrongParents(tf.BlockIDs("block3", "block4")))

	// issue block1
	{
		tf.IssueBlocks("block2").WaitUntilAllTasksProcessed()
		tf.AssertMissing(map[string]bool{
			"block1": true,
			"block2": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {},
		})

		tf.AssertBlock("block1", func(block *Block) {
			assert.True(t, block.Block.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block2
	{
		tf.IssueBlocks("block1").WaitUntilAllTasksProcessed()

		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {},
		})

		tf.AssertBlock("block1", func(block *Block) {
			assert.False(t, block.Block.IssuingTime().IsZero(), "block should be attached")
		})
	}

	// issue block4
	{
		tf.IssueBlocks("block4").WaitUntilAllTasksProcessed()
		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": true,
			"block4": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
			"block3": false,
			"block4": false,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {"block4"},
			"block3": {"block4"},
			"block4": {},
		})

		tf.AssertBlock("block3", func(block *Block) {
			assert.True(t, block.Block.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block5
	{
		tf.IssueBlocks("block5").WaitUntilAllTasksProcessed()
		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": true,
			"block4": false,
			"block5": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2"},
			"block2": {"block4"},
			"block3": {"block4", "block5"},
			"block4": {"block5"},
			"block5": {},
		})

		tf.AssertBlock("block3", func(block *Block) {
			assert.True(t, block.Block.IssuingTime().IsZero(), "block should not be attached")
		})
	}

	// issue block3
	{
		tf.IssueBlocks("block3").WaitUntilAllTasksProcessed()
		tf.AssertMissing(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertSolid(map[string]bool{
			"block1": true,
			"block2": true,
			"block3": true,
			"block4": true,
			"block5": true,
		})

		tf.AssertInvalid(map[string]bool{
			"block1": false,
			"block2": false,
			"block3": false,
			"block4": false,
			"block5": false,
		})

		tf.AssertStrongChildren(map[string][]string{
			"block1": {"block2", "block3"},
			"block2": {"block4", "block3"},
			"block3": {"block4", "block5"},
			"block4": {"block5"},
			"block5": {},
		})

		tf.AssertBlock("block3", func(block *Block) {
			assert.False(t, block.Block.IssuingTime().IsZero(), "block should be attached")
		})
	}
}

func TestTangle_AttachBlockTwice_1(t *testing.T) {
	tf := NewTestFramework(t)
	defer tf.Shutdown()
	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))
	storage := tf.Tangle.memStorage.Get(tf.Block("block2").ID().Index(), true)
	storage.Lock()
	var (
		wasAttached1 bool
		wasAttached2 bool
		err1         error
		err2         error
		started      uint8
	)

	event.Loop.Submit(func() {
		started++
		_, wasAttached1, err1 = tf.Tangle.Attach(tf.Block("block2"))
	})
	event.Loop.Submit(func() {
		started++
		_, wasAttached2, err2 = tf.Tangle.Attach(tf.Block("block2"))
	})

	time.Sleep(1 * time.Second)

	assert.Eventually(t, func() bool {
		if started == 2 {
			storage.Unlock()
			return true
		}
		return false
	}, time.Second*10, time.Millisecond*10)

	tf.WaitUntilAllTasksProcessed()

	assert.NoError(t, err1, "should not return an error")
	assert.NoError(t, err2, "should not return an error")
	assert.True(t, wasAttached1 != wasAttached2, "only one of the two should have been attached")
}

func TestTangle_AttachBlockTwice_2(t *testing.T) {
	tf := NewTestFramework(t)
	defer tf.Shutdown()
	tf.CreateBlock("block1")
	tf.CreateBlock("block2", models.WithStrongParents(tf.BlockIDs("block1")))

	_, wasAttached, err := tf.Tangle.Attach(tf.Block("block2"))
	assert.NoError(t, err, "should not return an error")
	assert.True(t, wasAttached, "should have been attached")

	_, wasAttached, err = tf.Tangle.Attach(tf.Block("block2"))
	assert.NoError(t, err, "should not return an error")
	assert.False(t, wasAttached, "should not have been attached")

	tf.WaitUntilAllTasksProcessed()

	assert.NoError(t, err, "should not return an error")
}

func TestTangle_Shutdown(t *testing.T) {
	testFramework := NewTestFramework(t)
	testFramework.Tangle.Shutdown()

	newBlock := testFramework.CreateBlock("block")

	_, _, err := testFramework.Tangle.Attach(newBlock)
	assert.Error(t, err, "should not be able to attach a block after shutdown")

	_, exists := testFramework.Tangle.Block(newBlock.ID())
	assert.False(t, exists, "block should not be in the tangle")

	wasUpdated := testFramework.Tangle.SetInvalid(&Block{})
	assert.False(t, wasUpdated, "block should not be updated")
}

// This test prepares blocks accross different epochs and tries to attach them in reverse order to a pruned tangle.
// At the end of the test only blocks from non-pruned epochs should be attached and marked as invalid.
func TestTangle_AttachInvalid(t *testing.T) {
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 0 {
			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			), alias
		}
		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)),
		), alias
	}

	// Prune tangle.
	assert.EqualValues(t, -1, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be 0")
	tf.Tangle.Prune(epochCount / 2)
	tf.WaitUntilAllTasksProcessed()
	assert.EqualValues(t, epochCount/2, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/2")

	blocks := make([]*models.Block, epochCount)
	expectedMissing := make(map[string]bool, epochCount)
	expectedInvalid := make(map[string]bool, epochCount)
	expectedSolid := make(map[string]bool, epochCount)

	// Prepare blocks and their expected states.
	for i := 0; i < epochCount; i++ {
		var alias string
		blocks[i], alias = createNewBlock(i, "")
		if i > 50 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = true
			expectedSolid[alias] = false
		} else if i == 50 {
			expectedMissing[alias] = true
			expectedInvalid[alias] = true
			expectedSolid[alias] = false
		}
	}

	// Issue the first bunch of blocks. Those should be marked as invalid when the block with pruned parents is attached.
	{
		for i := len(blocks) - 11; i >= 0; i-- {
			_, wasAttached, err := tf.Tangle.Attach(blocks[i])
			if blocks[i].ID().Index()-1 > tf.Tangle.maxDroppedEpoch {
				assert.True(t, wasAttached, "block should be attached")
				assert.NoError(t, err, "should not be able to attach a block after shutdown")
				continue
			}

			assert.False(t, wasAttached, "block should not be attached")
			assert.Error(t, err, "should not be able to attach a block to a pruned epoch")
		}
		tf.WaitUntilAllTasksProcessed()

		tf.AssertSolidCount(0, "should not have any solid blocks")
		tf.AssertInvalidCount(epochCount/2-10, "should have invalid blocks")
	}

	// Issue the second bunch of blocks. Those should be marked as invalid when the block attaches to a previously invalid block.
	{
		for i := len(blocks) - 1; i >= len(blocks)-10; i-- {
			_, wasAttached, err := tf.Tangle.Attach(blocks[i])
			assert.True(t, wasAttached, "block should be attached")
			assert.NoError(t, err, "should not be able to attach a block after shutdown")
		}
		tf.WaitUntilAllTasksProcessed()

		tf.AssertSolidCount(0, "should not have any solid blocks")
		tf.AssertInvalidCount(epochCount/2, "should have invalid blocks")
		tf.AssertMissing(expectedMissing)
		tf.AssertInvalid(expectedInvalid)
		tf.AssertSolid(expectedSolid)
	}
}

// This test creates two chains of blocks from the genesis (1 block per epoch in each chain). The first chain is solid, the second chain is not.
// When pruning the tangle, the first chain should be pruned but not marked as invalid by the causal order component, while the other should be marked as invalid.
func TestTangle_Prune(t *testing.T) {
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *models.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 0 {
			return tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			), alias
		}
		return tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)),
		), alias
	}

	assert.EqualValues(t, -1, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be 0")

	expectedMissing := make(map[string]bool, epochCount)
	expectedInvalid := make(map[string]bool, epochCount)
	expectedSolid := make(map[string]bool, epochCount)

	// Attach solid blocks
	for i := 0; i < epochCount; i++ {
		block, alias := createNewBlock(i, "")

		_, wasAttached, err := tf.Tangle.Attach(block)

		if i > epochCount/4 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = false
			expectedSolid[alias] = true
		}

		assert.True(t, wasAttached, "block should be attached")
		assert.NoError(t, err, "should not be able to attach a block after shutdown")
	}

	// Attach a blocks that are not solid (skip the first one in the chain)
	for i := 0; i < epochCount; i++ {
		blk, alias := createNewBlock(i, "-orphan")

		if i == 0 {
			continue
		}
		_, wasAttached, err := tf.Tangle.Attach(blk)

		if i > epochCount/2 {
			expectedMissing[alias] = false
			expectedInvalid[alias] = true
			expectedSolid[alias] = false
		}

		assert.True(t, wasAttached, "block should be attached")
		assert.NoError(t, err, "should not be able to attach a block after shutdown")
	}
	tf.WaitUntilAllTasksProcessed()

	tf.AssertSolidCount(epochCount, "should have all solid blocks")

	_, exists := tf.Tangle.Block(tf.Block("blk-0").ID())
	assert.True(t, exists, "block should be in the tangle")

	tf.Tangle.Prune(epochCount / 4)
	tf.WaitUntilAllTasksProcessed()

	assert.EqualValues(t, epochCount/4, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/4")

	// All orphan blocks should be marked as invalid due to invalidity propagation.
	tf.AssertInvalidCount(epochCount, "should have invalid blocks")

	tf.Tangle.Prune(epochCount / 10)
	tf.WaitUntilAllTasksProcessed()
	assert.EqualValues(t, epochCount/4, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/4")

	tf.Tangle.Prune(epochCount / 2)
	tf.WaitUntilAllTasksProcessed()
	assert.EqualValues(t, epochCount/2, tf.Tangle.maxDroppedEpoch, "maxDroppedEpoch should be epochCount/2")

	_, exists = tf.Tangle.Block(tf.Block("blk-0").ID())
	assert.False(t, exists, "block should not be in the tangle")
}

func TestTangle_MissingBlocks(t *testing.T) {
	const (
		blockCount  = 10000
		tangleWidth = 500
		storeDelay  = 0 * time.Millisecond
	)

	tf := NewTestFramework(t)
	defer tf.Shutdown()

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
		blk := tf.CreateBlock(fmt.Sprintf("msg-%d", idx), models.WithStrongParents(tf.BlockIDs(strongParents...)))
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

	tf.Tangle.Events.BlockMissing.Attach(event.NewClosure(func(metadata *Block) {
		time.Sleep(storeDelay)

		_, _, err := tf.Tangle.Attach(blocks[metadata.ID()])
		assert.NoError(t, err, "should be able to attach a block")
	}))

	// issue tips to start solidification
	tips.ForEach(func(key models.BlockID, _ models.BlockID) {
		_, _, err := tf.Tangle.Attach(blocks[key])
		assert.NoError(t, err, "should be able to attach a block")
	})

	// wait until all blocks are solidified
	tf.WaitUntilAllTasksProcessed()

	tf.AssertStoredCount(blockCount, "should have all blocks")
	tf.AssertInvalidCount(0, "should have no invalid blocks")
	tf.AssertSolidCount(blockCount, "should have all solid blocks")
	tf.AssertMissingCount(0, "should have no missing blocks")
}
